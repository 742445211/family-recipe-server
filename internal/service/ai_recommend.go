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
	recipes   *RecipeService
	orders    *OrderService
}

func NewAIRecommendService(db *gorm.DB, store cache.Store, ai *AIService, ctxSvc *AIContextService, rateLimit *AIRateLimitService) *AIRecommendService {
	return &AIRecommendService{
		db:        db,
		store:     store,
		ai:        ai,
		ctxSvc:    ctxSvc,
		rateLimit: rateLimit,
		recipes:   NewRecipeService(db),
		orders:    NewOrderService(db),
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
func (s *AIRecommendService) Recommend(ctx context.Context, familyID, userID uint64) (*AIRecommendResult, error) {
	var rl *RateLimitStatus
	if s.rateLimit != nil {
		st, err := s.rateLimit.CheckAndConsume(ctx, userID)
		if err != nil {
			return &AIRecommendResult{RateLimit: st}, err
		}
		rl = st
	}

	actx, err := s.ctxSvc.Build(familyID)
	if err != nil {
		return nil, err
	}

	raw, err := s.ai.RecommendStructured(actx, s.recommendCount())
	if err != nil {
		return nil, err
	}
	inputs, err := parseAIRecommendJSON(raw)
	if err != nil {
		return nil, err
	}

	nameToID := s.familyRecipeNameMap(familyID)
	batchID := uuid.New().String()
	ttl := s.recommendTTL()
	summaries := make([]AIRecommendItemSummary, 0, len(inputs))

	for _, in := range inputs {
		itemID := uuid.New().String()
		var existingID *uint64
		if id, ok := nameToID[strings.TrimSpace(in.Name)]; ok {
			existingID = &id
		}
		draft := AIRecipeDraft{
			ItemID:           itemID,
			BatchID:          batchID,
			FamilyID:         familyID,
			Name:             strings.TrimSpace(in.Name),
			Category:         defaultStr(in.Category, "其他"),
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
			ItemID: itemID, Name: draft.Name, Reason: draft.Reason, ExistingRecipeID: existingID,
		})
	}

	_ = s.store.SetJSON(ctx, aiBatchKey(familyID, batchID), map[string]interface{}{
		"batch_id": batchID, "item_ids": pluckItemIDs(summaries), "created_at": time.Now(),
	}, ttl)

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
	r := &model.Recipe{
		Name:        draft.Name,
		Category:    draft.Category,
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
		meal = "dinner"
	}
	return s.orders.Add(familyID, recipeID, meal, userID, date, req.Note, qty)
}

func parseAIRecommendJSON(raw string) ([]AIRecommendItemInput, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	var resp aiRecommendLLMResponse
	if err := json.Unmarshal([]byte(s), &resp); err != nil {
		return nil, fmt.Errorf("AI JSON 解析失败: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, errors.New("AI 未返回菜品")
	}
	return resp.Items, nil
}

func defaultStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return strings.TrimSpace(s)
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
