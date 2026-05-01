package service

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recipe-server/config"
)

func init() {
	// 加载配置
	if err := config.Load("../../config.yaml"); err != nil {
		panic("加载配置失败: " + err.Error())
	}
}

// TestOSSConnect 测试 OSS 连接和上传
func TestOSSConnect(t *testing.T) {
	cfg := config.AppConfig.OSS
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		t.Skip("OSS 未配置，跳过测试")
	}

	// 用内存中的文件模拟上传
	content := []byte("test-image-content")
	reader := bytes.NewReader(content)

	key := fmt.Sprintf("test/%d.txt", time.Now().Unix())
	url, err := uploadToOSS(cfg, key, reader, int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("OSS 上传失败: %v", err)
	}
	t.Logf("上传成功: %s", url)
}

// TestSaveImageOSS 测试完整上传流程
func TestSaveImageOSS(t *testing.T) {
	if config.AppConfig.OSS.Endpoint == "" {
		t.Skip("OSS 未配置")
	}

	// 创建临时文件模拟 multipart upload
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jpg")
	if err := os.WriteFile(tmpFile, []byte("fake-jpeg-data"), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// 构造 multipart.FileHeader 太复杂，直接测底层
	key, url, err := saveImageReader(f, "test.jpg")
	if err != nil {
		t.Fatalf("保存图片失败: %v", err)
	}
	t.Logf("key=%s url=%s", key, url)
	if key == "" {
		t.Error("key 不能为空")
	}
	if url == "" {
		t.Error("url 不能为空")
	}
}

// saveImageReader 测试辅助函数
func saveImageReader(r io.Reader, filename string) (string, string, error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	key := fmt.Sprintf("recipe/%d%s", time.Now().UnixNano(), ext)

	data, err := io.ReadAll(r)
	if err != nil {
		return "", "", err
	}

	cfg := config.AppConfig.OSS
	url, err := uploadToOSS(cfg, key, bytes.NewReader(data), int64(len(data)), "image/jpeg")
	if err != nil {
		return "", "", err
	}
	return key, url, nil
}
