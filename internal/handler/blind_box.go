package handler

import (
	"errors"
	"net/http"

	"recipe-server/config"
	"recipe-server/internal/middleware"
	"recipe-server/internal/service"
	"recipe-server/pkg/dateutil"

	"github.com/gin-gonic/gin"
)

type blindBoxDrawReq struct {
	Date       string   `json:"date"`
	MealType   string   `json:"meal_type"`
	ExcludeIDs []uint64 `json:"exclude_ids"`
}

// DrawBlindBox POST /api/orders/blind-box/draw
func (h *OrderHandler) DrawBlindBox(c *gin.Context) {
	if config.AppConfig != nil && !config.AppConfig.BlindBoxEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "盲盒功能未开启"})
		return
	}
	if h.blindBox == nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "盲盒功能未开启"})
		return
	}

	var req blindBoxDrawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	date := req.Date
	if date == "" {
		date = today()
	} else {
		date = dateutil.FormatYMD(date)
	}
	mealType := req.MealType
	if mealType == "" {
		mealType = "dinner"
	}

	res, err := h.blindBox.Draw(c.Request.Context(), middleware.GetFamilyID(c), middleware.GetUserID(c), date, mealType, req.ExcludeIDs)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBlindBoxRateLimit):
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  err.Error(),
				"data": gin.H{"rate_limit": res.RateLimit},
			})
		case errors.Is(err, service.ErrBlindBoxNoCandidates):
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "data": gin.H{"rate_limit": res.RateLimit}})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "抽取失败"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": res})
}
