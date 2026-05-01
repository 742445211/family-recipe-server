package handler

import (
	"net/http"
	"strings"

	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AIHandler struct {
	db      *gorm.DB
	aiSvc   *service.AIService
}

func NewAIHandler(db *gorm.DB) *AIHandler {
	return &AIHandler{db: db, aiSvc: service.NewAIService()}
}

func (h *AIHandler) Recommend(c *gin.Context) {
	familyID := middleware.GetFamilyID(c)

	// 获取家庭已有菜谱名称
	var recipes []model.Recipe
	h.db.Where("family_id = ?", familyID).Select("name").Find(&recipes)
	names := make([]string, len(recipes))
	for i, r := range recipes {
		names[i] = r.Name
	}

	// 获取最近5条点菜记录的菜名（使用 daily_orders 表）
	var orders []model.DailyOrder
	h.db.Preload("Recipe").
		Where("family_id = ?", familyID).
		Order("created_at DESC").
		Limit(5).
		Find(&orders)

	historyNames := []string{}
	for _, o := range orders {
		if o.Recipe != nil {
			historyNames = append(historyNames, o.Recipe.Name)
		}
	}
	historySummary := "暂无历史记录"
	if len(historyNames) > 0 {
		historySummary = "最近点过的菜：" + strings.Join(historyNames, "、")
	}

	result, err := h.aiSvc.Recommend(names, historySummary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "AI推荐失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"recommendation": result,
	}})
}
