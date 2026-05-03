// Package handler 提供 HTTP 请求处理器（Gin handlers）。
//
// 本文件 (upload.go) 负责文件上传相关接口：
//   - 上传图片文件（存储到 OSS/本地，返回访问 URL）
package handler

import (
	"net/http"

	"recipe-server/internal/middleware"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
)

// UploadHandler 文件上传处理器。
// 无状态结构体（不持有数据库连接），仅处理文件上传逻辑。
type UploadHandler struct{}

// Upload 上传图片文件接口。
//
// 路由：POST /api/upload（需认证）
//
// 功能：
//   接收 multipart/form-data 上传的图片文件，
//   存储到对象存储（OSS）或本地文件系统，返回文件标识 key 和访问 URL。
//
// 请求：
//   - Content-Type: multipart/form-data
//   - 表单字段 file: 图片文件（支持常见图片格式）
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"key":"uploads/xxx.jpg", "url":"https://..."}}
//   - 失败：{"code":400, "msg":"请选择文件"} / {"code":500, "msg":"..."}
func (h *UploadHandler) Upload(c *gin.Context) {
	// 验证用户登录状态（GetUserID 会在未认证时返回错误，由中间件拦截）
	_ = middleware.GetUserID(c)

	// 1. 从 multipart 表单中读取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择文件"})
		return
	}
	defer file.Close() // 确保文件句柄关闭

	// 2. 调用 service 层保存图片（上传到 OSS 或本地文件系统）
	key, url, err := service.SaveImage(file, header)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	// 3. 返回文件标识 key 和访问 URL
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"key": key, // 文件存储标识（如 "uploads/2026/05/xxx.jpg"）
		"url": url, // 文件完整访问 URL（如 "https://cdn.example.com/uploads/xxx.jpg"）
	}})
}
