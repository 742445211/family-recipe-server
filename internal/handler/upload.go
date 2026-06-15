// Package handler 提供 HTTP 请求处理器（Gin handlers）。
//
// 本文件 (upload.go) 负责文件上传相关接口：
//   - 上传图片文件（存储到 OSS/本地，返回访问 URL）
package handler

import (
	"net/http"
	"strconv"

	"recipe-server/internal/middleware"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
)

// UploadHandler 文件上传处理器。
type UploadHandler struct {
	ImageWorker *service.ImageWorkerService
}

// Upload 上传图片文件接口。
//
// 路由：POST /api/upload（需认证）
//
// 表单字段:
//   - file: 图片文件
//   - recipe_id: 可选，关联菜谱 ID（用于压缩后 DB key 同步）
func (h *UploadHandler) Upload(c *gin.Context) {
	_ = middleware.GetUserID(c)

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

	var recipeID uint64
	if raw := c.PostForm("recipe_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			recipeID = id
		}
	}

	if h.ImageWorker != nil {
		h.ImageWorker.DispatchCompress(key, url, recipeID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"key": key,
		"url": url,
	}})
}
