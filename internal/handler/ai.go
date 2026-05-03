// Package handler 提供 HTTP 请求处理器（Gin handlers）。
//
// 本文件 (ai.go) 负责 AI 智能推荐相关接口：
//   - 根据家庭已有菜谱和历史点菜记录，调用 AI 服务生成推荐。
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

// AIHandler AI 智能推荐处理器。
// 聚合家庭菜谱数据与点菜历史，调用 AIService 生成推荐结果。
type AIHandler struct {
	db    *gorm.DB          // 数据库连接，用于查询菜谱和点菜记录
	aiSvc *service.AIService // AI 推荐服务实例
}

// NewAIHandler 创建 AI 推荐处理器。
func NewAIHandler(db *gorm.DB) *AIHandler {
	return &AIHandler{db: db, aiSvc: service.NewAIService()}
}

// Recommend AI 智能推荐接口。
//
// 路由：POST /api/ai/recommend（需认证）
//
// 功能：
//   收集当前家庭的所有菜谱名称和最近 5 条点菜记录，
//   调用 AI 服务生成推荐菜品建议。
//
// 请求（无需 body）：
//   - 通过中间件获取 family_id（当前家庭）
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"recommendation":"..."}}
//   - 失败：{"code":500, "msg":"AI推荐失败: ..."}
func (h *AIHandler) Recommend(c *gin.Context) {
	// 从 JWT 上下文中获取当前家庭 ID
	familyID := middleware.GetFamilyID(c)

	// 1. 获取家庭已有菜谱名称列表（仅查询 name 列，减少数据传输）
	var recipes []model.Recipe
	h.db.Where("family_id = ?", familyID).Select("name").Find(&recipes)
	names := make([]string, len(recipes))
	for i, r := range recipes {
		names[i] = r.Name
	}

	// 2. 获取最近 5 条点菜记录的菜名（用于学习用户口味偏好）
	var orders []model.DailyOrder
	h.db.Preload("Recipe").                 // 预加载关联的菜谱信息
		Where("family_id = ?", familyID). // 仅当前家庭
		Order("created_at DESC").          // 按创建时间倒序
		Limit(5).                          // 最多取 5 条
		Find(&orders)

	// 3. 构建历史点菜摘要字符串，供 AI 分析口味偏好
	historyNames := []string{}
	for _, o := range orders {
		if o.Recipe != nil {
			historyNames = append(historyNames, o.Recipe.Name)
		}
	}
	historySummary := "暂无历史记录" // 无历史时的默认提示
	if len(historyNames) > 0 {
		historySummary = "最近点过的菜：" + strings.Join(historyNames, "、")
	}

	// 4. 调用 AI 服务获取推荐结果
	result, err := h.aiSvc.Recommend(names, historySummary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "AI推荐失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"recommendation": result,
	}})
}
