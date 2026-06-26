package handler

import (
	"errors"
	"net/http"

	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
)

// respondSecCheck 处理内容安全检测结果；若已写响应则返回 true。
func respondSecCheck(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, service.ErrContentUnsafe) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": service.ErrContentUnsafe.Error()})
		return true
	}
	if errors.Is(err, service.ErrImageTooLargeForSecCheck) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return true
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "内容安全检查失败"})
	return true
}
