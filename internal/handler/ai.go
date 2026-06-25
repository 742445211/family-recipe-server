// Package handler - AI 智能推荐接口。
package handler

import (
	"errors"
	"net/http"

	"recipe-server/config"
	"recipe-server/internal/middleware"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
)

// AIHandler AI 推荐处理器。
type AIHandler struct {
	recommend *service.AIRecommendService
	weather   *service.WeatherService
}

// NewAIHandler 创建 AI 处理器。
func NewAIHandler(recommend *service.AIRecommendService, weather *service.WeatherService) *AIHandler {
	return &AIHandler{recommend: recommend, weather: weather}
}

// aiRecommendDisabled 当 AI 推荐未开启时写 403 并返回 true。
func aiRecommendDisabled(c *gin.Context) bool {
	if config.AppConfig == nil || !config.AppConfig.AIRecommendEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "AI推荐功能未开启"})
		return true
	}
	return false
}

func aiRequireFamily(c *gin.Context) (uint64, bool) {
	return requireFamilyID(c)
}

// Recommend POST /api/ai/recommend
func (h *AIHandler) Recommend(c *gin.Context) {
	if aiRecommendDisabled(c) {
		return
	}
	familyID, ok := aiRequireFamily(c)
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)
	mealType := c.Query("meal_type")

	result, err := h.recommend.Recommend(c.Request.Context(), familyID, userID, mealType)
	if errors.Is(err, service.ErrRateLimitExceeded) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code": 429,
			"msg":  "AI推荐次数已达上限，请稍后再试",
			"data": result.RateLimit,
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "AI推荐失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": result})
}

// GetItem GET /api/ai/items/:item_id
func (h *AIHandler) GetItem(c *gin.Context) {
	if aiRecommendDisabled(c) {
		return
	}
	itemID := c.Param("item_id")
	familyID, ok := aiRequireFamily(c)
	if !ok {
		return
	}
	draft, err := h.recommend.GetItem(c.Request.Context(), itemID, familyID)
	if errors.Is(err, service.ErrAIItemNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}
	if errors.Is(err, service.ErrAIItemForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": draft})
}

type importRecipeReq struct{}

// ImportRecipe POST /api/ai/items/:item_id/import-recipe
func (h *AIHandler) ImportRecipe(c *gin.Context) {
	if aiRecommendDisabled(c) {
		return
	}
	itemID := c.Param("item_id")
	familyID, ok := aiRequireFamily(c)
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)
	rec, err := h.recommend.ImportRecipe(c.Request.Context(), itemID, familyID, userID)
	if errors.Is(err, service.ErrAIItemNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}
	if errors.Is(err, service.ErrAIItemForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": err.Error()})
		return
	}
	if errors.Is(err, service.ErrRecipeExists) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "该菜已在家庭菜谱库中"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": rec})
}

// AddOrder POST /api/ai/items/:item_id/add-order
func (h *AIHandler) AddOrder(c *gin.Context) {
	if aiRecommendDisabled(c) {
		return
	}
	itemID := c.Param("item_id")
	familyID, ok := aiRequireFamily(c)
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)
	var req service.AddOrderFromAIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	order, err := h.recommend.AddOrderFromItem(c.Request.Context(), itemID, familyID, userID, req)
	if errors.Is(err, service.ErrAIItemNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}
	if errors.Is(err, service.ErrAIItemForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": err.Error()})
		return
	}
	if err != nil {
		writeOrderAddError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": order})
}

// Weather GET /api/weather
func (h *AIHandler) Weather(c *gin.Context) {
	if h.weather == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": nil})
		return
	}
	snap, err := h.weather.GetDefault(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取天气失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": snap})
}
