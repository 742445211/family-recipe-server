package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"recipe-server/config"
)

// SaveImage 保存上传图片到本地 OSS 目录，返回 key 和 URL
// 后续可替换为真正的阿里云 OSS SDK
func SaveImage(file multipart.File, header *multipart.FileHeader) (string, string, error) {
	uploadDir := "/www/uploads/recipe"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", "", fmt.Errorf("创建上传目录失败: %w", err)
	}

	ext := filepath.Ext(header.Filename)
	key := fmt.Sprintf("recipe/%d%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join(uploadDir, fmt.Sprintf("%d%s", time.Now().UnixNano(), ext))

	dst, err := os.Create(savePath)
	if err != nil {
		return "", "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", "", fmt.Errorf("保存文件失败: %w", err)
	}

	// 如果有自定义域名则用，否则用本地URL
	baseURL := config.AppConfig.OSS.CustomDomain
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s/uploads/recipe", "localhost:8080")
	}

	url := fmt.Sprintf("%s/%d%s", baseURL, time.Now().UnixNano(), ext)
	return key, url, nil
}
