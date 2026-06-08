// Package handler - 应用级公开接口（功能开关等，不含敏感配置）。
package handler

import (
	"net/http"

	"recipe-server/config"

	"github.com/gin-gonic/gin"
)

// AppHandler 应用功能开关等公开信息。
type AppHandler struct{}

// NewAppHandler 创建应用处理器。
func NewAppHandler() *AppHandler {
	return &AppHandler{}
}

// Features GET /api/app/features
// 返回前端可见的功能开关（无需登录）。
func (h *AppHandler) Features(c *gin.Context) {
	aiRecommend := false
	if config.AppConfig != nil {
		aiRecommend = config.AppConfig.AIRecommendEnabled()
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": gin.H{
			"ai_recommend": aiRecommend,
		},
	})
}
