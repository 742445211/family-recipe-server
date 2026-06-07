package service

import (
	"context"
	"fmt"
	"strings"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

// AIRecommendContext 推荐上下文。
type AIRecommendContext struct {
	RecipeNames  []string
	OrderHistory []string
	WeatherLine  string
}

// AIContextService 组装 AI 推荐上下文。
type AIContextService struct {
	db      *gorm.DB
	orders  *OrderService
	weather *WeatherService
}

func NewAIContextService(db *gorm.DB, weather *WeatherService) *AIContextService {
	return &AIContextService{
		db:      db,
		orders:  NewOrderService(db),
		weather: weather,
	}
}

// Build 收集家庭菜名、点菜历史、天气摘要。
func (s *AIContextService) Build(familyID uint64) (*AIRecommendContext, error) {
	var recipes []model.Recipe
	if err := s.db.Where("family_id = ?", familyID).Select("name").Find(&recipes).Error; err != nil {
		return nil, err
	}
	names := make([]string, 0, len(recipes))
	seen := map[string]bool{}
	for _, r := range recipes {
		n := strings.TrimSpace(r.Name)
		if n != "" && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}

	history, err := s.orders.GetRecentOrderNames(familyID, 7, 21)
	if err != nil {
		return nil, err
	}

	weatherLine := "天气信息暂不可用"
	if s.weather != nil {
		weatherLine = s.weather.SummaryForPrompt(context.Background())
	}

	return &AIRecommendContext{
		RecipeNames:  names,
		OrderHistory: history,
		WeatherLine:  weatherLine,
	}, nil
}

// FormatHistorySummary 历史点菜文本摘要。
func FormatHistorySummary(names []string) string {
	if len(names) == 0 {
		return "暂无历史记录"
	}
	return "最近点过的菜：" + strings.Join(names, "、")
}

// FormatContextBlock 完整上下文块（注入 Prompt）。
func FormatContextBlock(ctx *AIRecommendContext) string {
	if ctx == nil {
		return ""
	}
	return fmt.Sprintf("家庭已有菜谱：%v\n历史点菜：%s\n当前天气：%s",
		ctx.RecipeNames, FormatHistorySummary(ctx.OrderHistory), ctx.WeatherLine)
}
