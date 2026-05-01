package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"time"

	"recipe-server/config"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// SaveImage 上传图片到阿里云 OSS，返回 key 和 URL
func SaveImage(file multipart.File, header *multipart.FileHeader) (string, string, error) {
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	key := fmt.Sprintf("recipe/%d%s", time.Now().UnixNano(), ext)

	cfg := config.AppConfig.OSS
	url, err := uploadToOSS(cfg, key, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		return "", "", fmt.Errorf("OSS上传失败: %w", err)
	}
	return key, url, nil
}

// uploadToOSS 底层的 OSS 上传
func uploadToOSS(cfg config.OSSConfig, key string, reader io.Reader, size int64, contentType string) (string, error) {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return "", fmt.Errorf("OSS连接失败: %w", err)
	}

	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return "", fmt.Errorf("获取Bucket失败: %w", err)
	}

	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}

	if err := bucket.PutObject(key, reader, options...); err != nil {
		return "", err
	}

	// 构建 URL
	if cfg.CustomDomain != "" {
		return fmt.Sprintf("%s/%s", cfg.CustomDomain, key), nil
	}
	return fmt.Sprintf("https://%s.%s/%s", cfg.Bucket, cfg.Endpoint, key), nil
}
