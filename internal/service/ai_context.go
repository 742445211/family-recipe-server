package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

// MealSlot 推荐餐次。Type 为英文标识（breakfast/lunch/dinner/supper），Name 为中文展示名。
type MealSlot struct {
	Type string
	Name string
}

// InferMealSlot 根据时刻推断当前应推荐的餐次：
// 5-10 点早餐、10-14 点午餐、14-21 点晚餐，其余（深夜/凌晨）为宵夜。
func InferMealSlot(t time.Time) MealSlot {
	switch h := t.Hour(); {
	case h >= 5 && h < 10:
		return MealSlot{Type: "breakfast", Name: "早餐"}
	case h >= 10 && h < 14:
		return MealSlot{Type: "lunch", Name: "午餐"}
	case h >= 14 && h < 21:
		return MealSlot{Type: "dinner", Name: "晚餐"}
	default:
		return MealSlot{Type: "supper", Name: "宵夜"}
	}
}

// NormalizeMealSlot 将中英文餐次输入规整为标准 MealSlot；无法识别返回 ok=false。
func NormalizeMealSlot(s string) (MealSlot, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "breakfast", "早餐", "早饭", "早":
		return MealSlot{Type: "breakfast", Name: "早餐"}, true
	case "lunch", "午餐", "午饭", "中餐", "午":
		return MealSlot{Type: "lunch", Name: "午餐"}, true
	case "dinner", "晚餐", "晚饭", "晚":
		return MealSlot{Type: "dinner", Name: "晚餐"}, true
	case "supper", "宵夜", "夜宵", "late_night", "midnight":
		return MealSlot{Type: "supper", Name: "宵夜"}, true
	}
	return MealSlot{}, false
}

// AIRecommendContext 推荐上下文。
type AIRecommendContext struct {
	RecipeNames  []string
	OrderHistory []string
	WeatherLine  string
	Meal         MealSlot // 本次推荐的目标餐次
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
		Meal:         InferMealSlot(time.Now()),
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
	existing := "（暂无，可自由推荐新菜）"
	if len(ctx.RecipeNames) > 0 {
		existing = strings.Join(ctx.RecipeNames, "、")
	}
	recent := FormatHistorySummary(ctx.OrderHistory)
	mealLine := ""
	if ctx.Meal.Name != "" {
		mealLine = fmt.Sprintf("当前餐次：%s（请重点推荐适合「%s」的菜品）\n", ctx.Meal.Name, ctx.Meal.Name)
	}
	return fmt.Sprintf(
		"%s家庭已有菜谱（禁止推荐以下菜名及近似菜名）：%s\n%s\n当前天气：%s",
		mealLine, existing, recent, ctx.WeatherLine,
	)
}
