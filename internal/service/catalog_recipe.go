package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"recipe-server/config"
	"recipe-server/internal/model"

	"gorm.io/gorm"
)

const (
	CatalogSourceAISearch    = "ai_search"
	CatalogSourceAIRecommend = "ai_recommend"
)

var ErrCatalogNameEmpty = errors.New("菜名不能为空")

// CatalogLookupOpts 查库/生成选项。
type CatalogLookupOpts struct {
	NewVariant bool
}

// CatalogLookupResult 查库或生成结果。
type CatalogLookupResult struct {
	Name        string                `json:"name"`
	Generated   bool                  `json:"generated"`
	Variants    []model.CatalogRecipe `json:"variants"`
	SelectedID  uint64                `json:"selected_id"`
	RateLimit   *RateLimitStatus      `json:"rate_limit,omitempty"`
}

// VariantSummary 已有做法摘要，供 AI 生成不同 variant。
type VariantSummary struct {
	VariantLabel string
	Category     string
	Summary      string
}

// CatalogRecipeService 全局菜谱库服务。
type CatalogRecipeService struct {
	db        *gorm.DB
	ai        *AIService
	rateLimit *AIRateLimitService
}

func NewCatalogRecipeService(db *gorm.DB, ai *AIService, rateLimit *AIRateLimitService) *CatalogRecipeService {
	return &CatalogRecipeService{db: db, ai: ai, rateLimit: rateLimit}
}

// NormalizeNameKey 规范化菜名用于精确检索。
func NormalizeNameKey(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		if unicode.IsSpace(r) {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CatalogContentHash 计算菜谱内容哈希（去重）。
func CatalogContentHash(ingredients, seasonings, steps string) string {
	raw := strings.TrimSpace(ingredients) + "|" + strings.TrimSpace(seasonings) + "|" + strings.TrimSpace(steps)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func defaultCatalogVariantLabel() string {
	if config.AppConfig != nil {
		return config.AppConfig.EffectiveCatalogDefaultVariantLabel()
	}
	return "经典做法"
}

// LookupByNameKey 按 name_key 精确查询全部做法。
func (s *CatalogRecipeService) LookupByNameKey(nameKey string) ([]model.CatalogRecipe, error) {
	var list []model.CatalogRecipe
	err := s.db.Where("name_key = ?", nameKey).
		Order("is_default DESC, use_count DESC, id ASC").
		Find(&list).Error
	return list, err
}

// GetByID 按 ID 获取全局菜谱。
func (s *CatalogRecipeService) GetByID(id uint64) (*model.CatalogRecipe, error) {
	var r model.CatalogRecipe
	if err := s.db.First(&r, id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

// IncrementUseCount 递增使用次数。
func (s *CatalogRecipeService) IncrementUseCount(id uint64) error {
	return s.db.Model(&model.CatalogRecipe{}).Where("id = ?", id).
		UpdateColumn("use_count", gorm.Expr("use_count + 1")).Error
}

func selectDefaultVariant(list []model.CatalogRecipe) *model.CatalogRecipe {
	if len(list) == 0 {
		return nil
	}
	for i := range list {
		if list[i].IsDefault {
			return &list[i]
		}
	}
	return &list[0]
}

// SaveFromAI 将 AI 输出写入全局库（hash 去重）。
func (s *CatalogRecipeService) SaveFromAI(input AIRecommendItemInput, source, variantLabel string) (*model.CatalogRecipe, error) {
	name := strings.TrimSpace(input.Name)
	nameKey := NormalizeNameKey(name)
	if nameKey == "" {
		return nil, ErrCatalogNameEmpty
	}
	hash := CatalogContentHash(defaultJSONArr(input.Ingredients), defaultJSONArr(input.Seasonings), defaultJSONArr(input.Steps))

	var existing model.CatalogRecipe
	err := s.db.Where("name_key = ? AND content_hash = ?", nameKey, hash).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var count int64
	if err := s.db.Model(&model.CatalogRecipe{}).Where("name_key = ?", nameKey).Count(&count).Error; err != nil {
		return nil, err
	}
	isDefault := count == 0
	if strings.TrimSpace(variantLabel) == "" {
		if isDefault {
			variantLabel = defaultCatalogVariantLabel()
		} else {
			variantLabel = "其他做法"
		}
	}

	rec := model.CatalogRecipe{
		Name:         name,
		NameKey:      nameKey,
		VariantLabel: strings.TrimSpace(variantLabel),
		IsDefault:    isDefault,
		Category:     defaultStr(input.Category, "其他"),
		Ingredients:  defaultJSONArr(input.Ingredients),
		Seasonings:   defaultJSONArr(input.Seasonings),
		Steps:        defaultJSONArr(input.Steps),
		CookTime:     input.CookTime,
		Difficulty:   normalizeDifficulty(input.Difficulty),
		Tips:         input.Tips,
		Source:       source,
		ContentHash:  hash,
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

// EnsureFromRecommendItem AI 推荐项写入全局库（不消耗 catalog 限流）。
func (s *CatalogRecipeService) EnsureFromRecommendItem(in AIRecommendItemInput) (*model.CatalogRecipe, error) {
	return s.SaveFromAI(in, CatalogSourceAIRecommend, "AI推荐")
}

func (s *CatalogRecipeService) variantSummaries(list []model.CatalogRecipe) []VariantSummary {
	out := make([]VariantSummary, 0, len(list))
	for _, v := range list {
		out = append(out, VariantSummary{
			VariantLabel: v.VariantLabel,
			Category:     v.Category,
			Summary:      fmt.Sprintf("%s/%s", v.Category, truncateRunes(v.Tips, 40)),
		})
	}
	return out
}

func truncateRunes(s string, n int) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) <= n {
		return string(rs)
	}
	return string(rs[:n]) + "…"
}

// LookupOrGenerate 查库或 AI 生成并入库。
func (s *CatalogRecipeService) LookupOrGenerate(ctx context.Context, userID uint64, name string, opts CatalogLookupOpts) (*CatalogLookupResult, error) {
	displayName := strings.TrimSpace(name)
	nameKey := NormalizeNameKey(displayName)
	if nameKey == "" {
		return nil, ErrCatalogNameEmpty
	}

	existing, err := s.LookupByNameKey(nameKey)
	if err != nil {
		return nil, err
	}

	needAI := len(existing) == 0 || opts.NewVariant
	var rl *RateLimitStatus

	if needAI {
		if s.rateLimit != nil {
			st, err := s.rateLimit.Peek(ctx, AIRateLimitScopeCatalog, userID)
			if err != nil {
				return nil, err
			}
			if s.rateLimit.cfg(AIRateLimitScopeCatalog).Enabled && st.Remaining <= 0 {
				st.RetryAfterSec = st.ResetAfterSec
				return &CatalogLookupResult{Name: displayName, RateLimit: st}, ErrCatalogRateLimitExceeded
			}
			st, err = s.rateLimit.CheckAndConsume(ctx, AIRateLimitScopeCatalog, userID)
			if err != nil {
				if errors.Is(err, ErrCatalogRateLimitExceeded) {
					st.RetryAfterSec = st.ResetAfterSec
					return &CatalogLookupResult{Name: displayName, RateLimit: st}, err
				}
				return nil, err
			}
			rl = st
		}
		if s.ai == nil {
			return nil, errors.New("AI 服务未配置")
		}
		summaries := s.variantSummaries(existing)
		input, err := s.ai.GenerateRecipeByName(displayName, summaries)
		if err != nil {
			return nil, err
		}
		label := ""
		if opts.NewVariant && strings.TrimSpace(input.Category) != "" {
			label = input.Category + "版"
		}
		if _, err := s.SaveFromAI(input, CatalogSourceAISearch, label); err != nil {
			return nil, err
		}
		existing, err = s.LookupByNameKey(nameKey)
		if err != nil {
			return nil, err
		}
	} else if s.rateLimit != nil {
		st, err := s.rateLimit.Peek(ctx, AIRateLimitScopeCatalog, userID)
		if err != nil {
			return nil, err
		}
		rl = st
	}

	selected := selectDefaultVariant(existing)
	if selected == nil {
		return nil, errors.New("未找到菜谱")
	}
	_ = s.IncrementUseCount(selected.ID)

	return &CatalogLookupResult{
		Name:       displayName,
		Generated:  needAI,
		Variants:   existing,
		SelectedID: selected.ID,
		RateLimit:  rl,
	}, nil
}
