package handler

import (
	"net/http"

	"recipe-server/internal/middleware"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct{}

func (h *UploadHandler) Upload(c *gin.Context) {
	_ = middleware.GetUserID(c) // 验证登录

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择文件"})
		return
	}
	defer file.Close()

	key, url, err := service.SaveImage(file, header)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"key": key,
		"url": url,
	}})
}
