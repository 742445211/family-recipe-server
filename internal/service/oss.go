// Package service - 阿里云 OSS 对象存储服务。
//
// 本文件实现图片上传到阿里云 OSS（Object Storage Service）的功能，
// 用于菜谱封面图等静态资源的云端存储与 CDN 加速分发。
//
// 上传流程：
//  1. 接收 HTTP multipart/form-data 中的图片文件
//  2. 根据原始文件扩展名生成 OSS 对象 key（格式：recipe/{UnixNano时间戳}{ext}）
//  3. 调用 OSS PutObject API 上传文件
//  4. 返回 OSS key 和可访问的完整 URL
//
// 依赖：github.com/aliyun/aliyun-oss-go-sdk/oss
// 配置：config.AppConfig.OSS（含 Endpoint、AccessKeyID、AccessKeySecret、Bucket、CustomDomain）
package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"recipe-server/config"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

const maxUploadBytes = 10 << 20 // 10MB

var allowedUploadExts = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".webp": {}, ".gif": {},
}

// ValidateUploadFile 校验上传文件大小与扩展名。
func ValidateUploadFile(header *multipart.FileHeader) error {
	if header.Size <= 0 {
		return fmt.Errorf("文件为空")
	}
	if header.Size > maxUploadBytes {
		return fmt.Errorf("文件过大，最大 10MB")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if _, ok := allowedUploadExts[ext]; !ok {
		return fmt.Errorf("不支持的图片格式")
	}
	return nil
}

// SaveImage 上传图片到阿里云 OSS，并返回存储 key 和可访问的完整 URL。
//
// 参数:
//   - file   multipart.File       - 上传的图片文件流（来自 HTTP 请求的 multipart form）
//   - header *multipart.FileHeader - 文件头信息（含文件名、大小、Content-Type）
//
// 返回值:
//   - string - OSS 对象存储 key（格式：recipe/{时间戳纳秒}{扩展名}，如 recipe/1714732800123456789.jpg）
//   - string - 图片完整访问 URL（优先使用自定义域名，否则拼接 OSS 默认域名）
//   - error  - OSS 上传失败时返回错误
//
// 说明:
//   - 文件扩展名从原始文件名提取，若无扩展名则默认使用 .jpg
//   - key 使用纳秒级时间戳保证唯一性，避免同名文件覆盖
func SaveImage(file multipart.File, header *multipart.FileHeader) (string, string, error) {
	if err := ValidateUploadFile(header); err != nil {
		return "", "", err
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))

	// 生成唯一 OSS key：recipe/{时间戳纳秒}{扩展名}
	key := fmt.Sprintf("recipe/%d%s", time.Now().UnixNano(), ext)

	// 调取底层 OSS 上传函数
	cfg := config.AppConfig.OSS
	url, err := uploadToOSS(cfg, key, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		return "", "", fmt.Errorf("OSS上传失败: %w", err)
	}
	return key, url, nil
}

// BuildObjectURL 根据 OSS key 构建可访问 URL。
func BuildObjectURL(key string) (string, error) {
	if config.AppConfig == nil {
		return "", fmt.Errorf("oss not configured")
	}
	cfg := config.AppConfig.OSS
	if cfg.Bucket == "" || cfg.Endpoint == "" {
		return "", fmt.Errorf("oss not configured")
	}
	if cfg.CustomDomain != "" {
		return fmt.Sprintf("%s/%s", cfg.CustomDomain, key), nil
	}
	return fmt.Sprintf("https://%s.%s/%s", cfg.Bucket, cfg.Endpoint, key), nil
}

// DeleteObject 删除 OSS 对象。
func DeleteObject(key string) error {
	cfg := config.AppConfig.OSS
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return fmt.Errorf("OSS连接失败: %w", err)
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return fmt.Errorf("获取Bucket失败: %w", err)
	}
	return bucket.DeleteObject(key)
}

// uploadToOSS 底层 OSS 对象上传函数，将文件流写入指定的 Bucket。
//
// 参数:
//   - cfg         config.OSSConfig - OSS 配置（Endpoint、AK、Bucket 等）
//   - key         string           - OSS 对象存储路径
//   - reader      io.Reader        - 文件内容读取器
//   - size        int64            - 文件大小（字节）
//   - contentType string           - 文件 MIME 类型（来自 HTTP Header，用于设置 OSS Content-Type）
//
// 返回值:
//   - string - 文件访问 URL（优先使用自定义域名，否则使用默认 OSS 域名）
//   - error  - OSS 客户端连接失败、Bucket 获取失败、或 PutObject 上传失败时返回错误
//
// OSS 操作流程:
//  1. oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret) 创建 OSS 客户端
//  2. client.Bucket(cfg.Bucket) 获取 Bucket 实例
//  3. bucket.PutObject(key, reader, options...) 上传文件内容
//  4. 根据配置构造访问 URL
//
// URL 构建规则:
//   - 若配置了 CustomDomain → https://{custom_domain}/{key}
//   - 否则 → https://{bucket}.{endpoint}/{key}
func uploadToOSS(cfg config.OSSConfig, key string, reader io.Reader, size int64, contentType string) (string, error) {
	// 创建 OSS 客户端（Endpoint + AK/SK 认证）
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return "", fmt.Errorf("OSS连接失败: %w", err)
	}

	// 获取目标 Bucket 实例
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return "", fmt.Errorf("获取Bucket失败: %w", err)
	}

	// 如果指定了 Content-Type，设置上传选项
	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}

	// 执行 PutObject 上传
	if err := bucket.PutObject(key, reader, options...); err != nil {
		return "", err
	}

	// 构建访问 URL：优先使用自定义域名（如 CDN），否则拼接 OSS 默认域名
	return BuildObjectURL(key)
}
