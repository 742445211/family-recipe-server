// Package service - AI 智能推荐（结构化 + Redis 草稿）。
//
// 聚合家庭菜谱、点菜历史、天气等上下文调用 LLM，批次结果写入 Redis；
// 用户可从草稿 import-recipe 入库或 add-order 直接点菜。recommend 与 catalog 限流独立计数。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"recipe-server/config"
	"recipe-server/internal/cache"
	"recipe-server/internal/model"
	"recipe-server/pkg/dateutil"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrAIItemNotFound   = errors.New("推荐菜品不存在或已过期")
	ErrAIItemForbidden  = errors.New("无权访问该推荐菜品")
	ErrRecipeExists     = errors.New("该菜已在家庭菜谱库中")
)

// AIRecommendItemInput LLM 返回的单项。
type AIRecommendItemInput struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	MealType    string `json:"meal_type"`
	Difficulty  string `json:"difficulty"`
	CookTime    int    `json:"cook_time"`
	Ingredients string `json:"ingredients"`
	Seasonings  string `json:"seasonings"`
	Steps       string `json:"steps"`
	Tips        string `json:"tips"`
	Reason      string `json:"reason"`
}

type aiRecommendLLMResponse struct {
	Items []AIRecommendItemInput `json:"items"`
}

// AIRecipeDraft Redis 中存储的完整菜谱草稿。
type AIRecipeDraft struct {
	ItemID            string  `json:"item_id"`
	BatchID           string  `json:"batch_id"`
	FamilyID          uint64  `json:"family_id"`
	Name              string  `json:"name"`
	Category          string  `json:"category"`
	MealType          string  `json:"meal_type"`
	Difficulty        string  `json:"difficulty"`
	CookTime          int     `json:"cook_time"`
	Ingredients       string  `json:"ingredients"`
	Seasonings        string  `json:"seasonings"`
	Steps             string  `json:"steps"`
	Tips              string  `json:"tips"`
	Reason            string  `json:"reason"`
	ExistingRecipeID  *uint64 `json:"existing_recipe_id,omitempty"`
}

// AIRecommendItemSummary 列表项摘要。
type AIRecommendItemSummary struct {
	ItemID           string  `json:"item_id"`
	Name             string  `json:"name"`
	MealType         string  `json:"meal_type"`
	Reason           string  `json:"reason"`
	ExistingRecipeID *uint64 `json:"existing_recipe_id,omitempty"`
}

// AIRecommendResult 推荐批次结果。
type AIRecommendResult struct {
	BatchID   string                   `json:"batch_id"`
	Items     []AIRecommendItemSummary `json:"items"`
	RateLimit *RateLimitStatus         `json:"rate_limit"`
}

// AddOrderFromAIRequest 从 AI 项点菜请求。
type AddOrderFromAIRequest struct {
	MealType string `json:"meal_type"`
	Date     string `json:"date"`
	Note     string `json:"note"`
	Quantity int    `json:"quantity"`
}

// AIRecommendService 结构化 AI 推荐与 Redis 缓存。
type AIRecommendService struct {
	db        *gorm.DB
	store     cache.Store
	ai        *AIService
	ctxSvc    *AIContextService
	rateLimit *AIRateLimitService
	recipes    *RecipeService
	orders     *OrderService
	categories *CategoryService
	catalog    *CatalogRecipeService
}

func NewAIRecommendService(db *gorm.DB, store cache.Store, ai *AIService, ctxSvc *AIContextService, rateLimit *AIRateLimitService) *AIRecommendService {
	catalog := NewCatalogRecipeService(db, ai, rateLimit)
	return &AIRecommendService{
		db:         db,
		store:      store,
		ai:         ai,
		ctxSvc:     ctxSvc,
		rateLimit:  rateLimit,
		recipes:    NewRecipeService(db),
		orders:     NewOrderService(db),
		categories: NewCategoryService(db),
		catalog:    catalog,
	}
}

func aiItemKey(itemID string) string {
	return "ai:item:" + itemID
}

func aiBatchKey(familyID uint64, batchID string) string {
	return fmt.Sprintf("ai:batch:%d:%s", familyID, batchID)
}

func (s *AIRecommendService) recommendTTL() time.Duration {
	h := 24
	if config.AppConfig != nil && config.AppConfig.AI.RecommendCacheTTLHours > 0 {
		h = config.AppConfig.AI.RecommendCacheTTLHours
	}
	return time.Duration(h) * time.Hour
}

func (s *AIRecommendService) recommendCount() int {
	if config.AppConfig != nil && config.AppConfig.AI.RecommendCount > 0 {
		return config.AppConfig.AI.RecommendCount
	}
	return 5
}

// Recommend 生成推荐并写入 Redis。
// mealType 为可选餐次覆盖（breakfast/lunch/dinner/supper 或中文），为空时按当前时间自动判断。
func (s *AIRecommendService) Recommend(ctx context.Context, familyID, userID uint64, mealType string) (*AIRecommendResult, error) {
	if s.rateLimit != nil {
		st, err := s.rateLimit.Peek(ctx, AIRateLimitScopeRecommend, userID)
		if err != nil {
			return nil, err
		}
		if s.rateLimit.cfg(AIRateLimitScopeRecommend).Enabled && st.Remaining <= 0 {
			st.RetryAfterSec = st.ResetAfterSec
			return &AIRecommendResult{RateLimit: st}, ErrRateLimitExceeded
		}
	}

	actx, err := s.ctxSvc.Build(familyID)
	if err != nil {
		return nil, err
	}
	// 调用方指定餐次时覆盖自动判断结果
	if ms, ok := NormalizeMealSlot(mealType); ok {
		actx.Meal = ms
	}

	nameToID := s.familyRecipeNameMap(familyID)
	inputs, err := s.fetchAIRecommendItems(actx, nameToID)
	if err != nil {
		return nil, err
	}

	batchID := uuid.New().String()
	ttl := s.recommendTTL()
	summaries := make([]AIRecommendItemSummary, 0, len(inputs))

	for _, in := range inputs {
		itemID := uuid.New().String()
		var existingID *uint64
		if id, ok := nameToID[strings.TrimSpace(in.Name)]; ok {
			existingID = &id
		}
		if s.catalog != nil {
			if _, err := s.catalog.EnsureFromRecommendItem(in); err != nil {
				return nil, err
			}
		}
		category, err := s.categories.Ensure(familyID, defaultStr(in.Category, "其他"))
		if err != nil {
			return nil, err
		}
		draft := AIRecipeDraft{
			ItemID:           itemID,
			BatchID:          batchID,
			FamilyID:         familyID,
			Name:             strings.TrimSpace(in.Name),
			Category:         category,
			MealType:         normalizeMealType(in.MealType, actx.Meal.Type),
			Difficulty:       normalizeDifficulty(in.Difficulty),
			CookTime:         in.CookTime,
			Ingredients:      defaultJSONArr(in.Ingredients),
			Seasonings:       defaultJSONArr(in.Seasonings),
			Steps:            defaultJSONArr(in.Steps),
			Tips:             in.Tips,
			Reason:           in.Reason,
			ExistingRecipeID: existingID,
		}
		if err := s.store.SetJSON(ctx, aiItemKey(itemID), draft, ttl); err != nil {
			return nil, err
		}
		summaries = append(summaries, AIRecommendItemSummary{
			ItemID: itemID, Name: draft.Name, MealType: draft.MealType, Reason: draft.Reason, ExistingRecipeID: existingID,
		})
	}

	if err := s.store.SetJSON(ctx, aiBatchKey(familyID, batchID), map[string]interface{}{
		"batch_id": batchID, "item_ids": pluckItemIDs(summaries), "created_at": time.Now(),
	}, ttl); err != nil {
		return nil, err
	}

	var rl *RateLimitStatus
	if s.rateLimit != nil {
		st, err := s.rateLimit.CheckAndConsume(ctx, AIRateLimitScopeRecommend, userID)
		if err != nil {
			return &AIRecommendResult{RateLimit: st}, err
		}
		rl = st
	}

	return &AIRecommendResult{BatchID: batchID, Items: summaries, RateLimit: rl}, nil
}

func pluckItemIDs(items []AIRecommendItemSummary) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ItemID
	}
	return ids
}

func (s *AIRecommendService) familyRecipeNameMap(familyID uint64) map[string]uint64 {
	var recipes []model.Recipe
	s.db.Where("family_id = ?", familyID).Select("id", "name").Find(&recipes)
	m := make(map[string]uint64, len(recipes))
	for _, r := range recipes {
		m[strings.TrimSpace(r.Name)] = r.ID
	}
	return m
}

// GetItem 从 Redis 读取草稿。
func (s *AIRecommendService) GetItem(ctx context.Context, itemID string, familyID uint64) (*AIRecipeDraft, error) {
	var draft AIRecipeDraft
	if err := s.store.GetJSON(ctx, aiItemKey(itemID), &draft); err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			return nil, ErrAIItemNotFound
		}
		return nil, err
	}
	if draft.FamilyID != familyID {
		return nil, ErrAIItemForbidden
	}
	return &draft, nil
}

// ImportRecipe 从 Redis 导入家庭菜谱。
func (s *AIRecommendService) ImportRecipe(ctx context.Context, itemID string, familyID, userID uint64) (*model.Recipe, error) {
	draft, err := s.GetItem(ctx, itemID, familyID)
	if err != nil {
		return nil, err
	}
	if draft.ExistingRecipeID != nil {
		return nil, ErrRecipeExists
	}
	if id, ok := s.familyRecipeNameMap(familyID)[draft.Name]; ok {
		return nil, fmt.Errorf("%w: id=%d", ErrRecipeExists, id)
	}
	category, err := s.categories.Ensure(familyID, draft.Category)
	if err != nil {
		return nil, err
	}
	r := &model.Recipe{
		Name:        draft.Name,
		Category:    category,
		Difficulty:  draft.Difficulty,
		CookTime:    draft.CookTime,
		Ingredients: draft.Ingredients,
		Seasonings:  draft.Seasonings,
		Steps:       draft.Steps,
		Tips:        draft.Tips,
		CreatorID:   userID,
		FamilyID:    familyID,
		IsPublic:    true,
	}
	if err := s.recipes.Create(r); err != nil {
		return nil, err
	}
	draft.ExistingRecipeID = &r.ID
	_ = s.store.SetJSON(ctx, aiItemKey(itemID), draft, s.recommendTTL())
	return r, nil
}

// AddOrderFromItem 从 Redis 导入（如需）并点菜。
func (s *AIRecommendService) AddOrderFromItem(ctx context.Context, itemID string, familyID, userID uint64, req AddOrderFromAIRequest) (*model.DailyOrder, error) {
	draft, err := s.GetItem(ctx, itemID, familyID)
	if err != nil {
		return nil, err
	}
	recipeID := uint64(0)
	if draft.ExistingRecipeID != nil {
		recipeID = *draft.ExistingRecipeID
	} else if id, ok := s.familyRecipeNameMap(familyID)[draft.Name]; ok {
		recipeID = id
	} else {
		rec, err := s.ImportRecipe(ctx, itemID, familyID, userID)
		if err != nil && !errors.Is(err, ErrRecipeExists) {
			return nil, err
		}
		if rec != nil {
			recipeID = rec.ID
		} else if id, ok := s.familyRecipeNameMap(familyID)[draft.Name]; ok {
			recipeID = id
		}
	}
	if recipeID == 0 {
		return nil, errors.New("无法确定菜谱 ID")
	}
	date := dateutil.FormatYMD(req.Date)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}
	meal := req.MealType
	if meal == "" {
		meal = draft.MealType
	}
	if meal == "" {
		meal = "dinner"
	}
	if ms, ok := NormalizeMealSlot(meal); ok {
		meal = ms.Type
	}
	return s.orders.Add(familyID, recipeID, meal, userID, date, req.Note, qty)
}

func (s *AIRecommendService) fetchAIRecommendItems(actx *AIRecommendContext, existing map[string]uint64) ([]AIRecommendItemInput, error) {
	count := s.recommendCount()
	minNew := count
	if minNew > 2 {
		minNew = count - 1
	}

	tryFetch := func(hint string) ([]AIRecommendItemInput, error) {
		raw, err := s.ai.recommendStructured(actx, count, hint)
		if err != nil {
			return nil, err
		}
		inputs, parseErr := parseAIRecommendJSON(raw)
		if parseErr != nil {
			return nil, parseErr
		}
		return filterNewDishesOnly(inputs, existing), nil
	}

	inputs, err := tryFetch("")
	if err == nil && len(inputs) >= minNew {
		return inputs, nil
	}

	// 解析失败、新菜不足或含大量重复时重试
	hint := "请全部推荐家庭菜谱库和最近点菜中都没有的新菜名，不要重复已有菜"
	inputs2, err2 := tryFetch(hint)
	if err2 == nil && len(inputs2) > 0 {
		return inputs2, nil
	}
	if err == nil && len(inputs) > 0 {
		return inputs, nil
	}
	if err2 != nil && err != nil {
		if strings.Contains(err.Error(), "AI JSON 解析失败") {
			return nil, err2
		}
		return nil, err
	}
	return inputs2, err2
}

// filterNewDishesOnly 去掉与家庭已有菜谱同名的推荐项。
func filterNewDishesOnly(inputs []AIRecommendItemInput, existing map[string]uint64) []AIRecommendItemInput {
	if len(existing) == 0 {
		return inputs
	}
	out := make([]AIRecommendItemInput, 0, len(inputs))
	for _, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			continue
		}
		if _, ok := existing[name]; ok {
			continue
		}
		out = append(out, in)
	}
	return out
}

type flexibleAIRecommendItem struct {
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	MealType    string          `json:"meal_type"`
	Difficulty  string          `json:"difficulty"`
	CookTime    int             `json:"cook_time"`
	Ingredients json.RawMessage `json:"ingredients"`
	Seasonings  json.RawMessage `json:"seasonings"`
	Steps       json.RawMessage `json:"steps"`
	Tips        string          `json:"tips"`
	Reason      string          `json:"reason"`
}

type flexibleAIRecommendResponse struct {
	Items []flexibleAIRecommendItem `json:"items"`
}

func parseAIRecommendJSON(raw string) ([]AIRecommendItemInput, error) {
	s := extractAIJSONPayload(raw)
	var resp flexibleAIRecommendResponse
	if err := json.Unmarshal([]byte(s), &resp); err != nil {
		return nil, fmt.Errorf("AI JSON 解析失败: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, errors.New("AI 未返回菜品")
	}
	out := make([]AIRecommendItemInput, 0, len(resp.Items))
	for _, it := range resp.Items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			continue
		}
		out = append(out, AIRecommendItemInput{
			Name:        name,
			Category:    it.Category,
			MealType:    it.MealType,
			Difficulty:  it.Difficulty,
			CookTime:    it.CookTime,
			Ingredients: normalizeJSONArrayField(it.Ingredients),
			Seasonings:  normalizeJSONArrayField(it.Seasonings),
			Steps:       normalizeJSONArrayField(it.Steps),
			Tips:        it.Tips,
			Reason:      it.Reason,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("AI 未返回有效菜品")
	}
	return out, nil
}

func extractAIJSONPayload(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	if start := strings.Index(s, "{"); start >= 0 {
		if end := strings.LastIndex(s, "}"); end > start {
			return s[start : end+1]
		}
	}
	return s
}

// normalizeJSONArrayField 兼容 LLM 返回 JSON 字符串或数组/对象，统一为 DB 用的 JSON 字符串。
func normalizeJSONArrayField(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return "[]"
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "[]"
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			s = strings.TrimSpace(s)
			if s == "" {
				return "[]"
			}
			return s
		}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func defaultStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return strings.TrimSpace(s)
}

// normalizeMealType 将 LLM 给出的餐次归一化为标准英文标识，识别失败时回落到推荐上下文的餐次。
func normalizeMealType(raw, fallback string) string {
	if ms, ok := NormalizeMealSlot(raw); ok {
		return ms.Type
	}
	return fallback
}

func normalizeDifficulty(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	switch d {
	case "easy", "简单":
		return "easy"
	case "hard", "困难":
		return "hard"
	default:
		return "medium"
	}
}

func defaultJSONArr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "[]"
	}
	return s
}
