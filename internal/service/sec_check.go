// Package service - 微信小程序内容安全（msgSecCheck）。
package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"recipe-server/config"
	"recipe-server/internal/model"
	"recipe-server/internal/service/wechattoken"
)

const (
	// SecCheckSceneProfile 资料类场景（菜谱、昵称、家庭名等）。
	SecCheckSceneProfile = 1
	// SecCheckSceneComment 评论/备注类场景（点菜备注等）。
	SecCheckSceneComment = 2
)

// ErrContentUnsafe 用户发布内容未通过微信内容安全检测。
var ErrContentUnsafe = errors.New("您发布的内容含违规信息，请修改后重试")

// ErrImageTooLargeForSecCheck 图片超过微信 imgSecCheck 同步检测上限（1MB）。
var ErrImageTooLargeForSecCheck = errors.New("图片过大，请压缩至1MB以内后上传")

const maxImgSecCheckBytes = 1 << 20 // 1MB，微信 img_sec_check 限制

var secCheckAPIBase = "https://api.weixin.qq.com"

// SetSecCheckAPIBaseForTest 测试专用：替换 msgSecCheck API 基址。
func SetSecCheckAPIBaseForTest(base string) {
	secCheckAPIBase = base
}

// SecCheckAPIBaseForTest 测试专用：读取 msgSecCheck API 基址。
func SecCheckAPIBaseForTest() string {
	return secCheckAPIBase
}

// SecCheckHTTPDoer HTTP 客户端接口。
type SecCheckHTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// SecCheckService 微信 msgSecCheck 封装。
type SecCheckService struct {
	token  wechattoken.Provider
	client SecCheckHTTPDoer
}

// NewSecCheckService 创建内容安全服务。
func NewSecCheckService(token wechattoken.Provider) *SecCheckService {
	return &SecCheckService{token: token, client: &http.Client{}}
}

// DefaultSecCheck 进程内默认实例。
var DefaultSecCheck = NewSecCheckService(wechattoken.SharedMiniProgramToken())

func (s *SecCheckService) enabled() bool {
	return config.AppConfig != nil && config.AppConfig.SecCheckEnabled()
}

// CheckTexts 对多条文本依次调用 msgSecCheck（空串跳过）。
func (s *SecCheckService) CheckTexts(openid string, scene int, texts ...string) error {
	if s == nil || !s.enabled() {
		return nil
	}
	openid = strings.TrimSpace(openid)
	if openid == "" {
		return fmt.Errorf("缺少 openid，无法完成内容安全检查")
	}
	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if err := s.checkOne(openid, scene, text); err != nil {
			return err
		}
	}
	return nil
}

// CheckRecipeUGC 检测菜谱创建/更新中的用户文本字段。
func (s *SecCheckService) CheckRecipeUGC(openid string, r *model.Recipe) error {
	if r == nil {
		return nil
	}
	texts := []string{r.Name, r.Category, r.Tips}
	texts = append(texts, extractStringsFromJSON(r.Ingredients, r.Seasonings, r.Steps)...)
	return s.CheckTexts(openid, SecCheckSceneProfile, texts...)
}

// CheckImage 调用微信 img_sec_check 检测图片（同步，最大 1MB）。
func (s *SecCheckService) CheckImage(openid string, data []byte, filename string) error {
	if s == nil || !s.enabled() {
		return nil
	}
	openid = strings.TrimSpace(openid)
	if openid == "" {
		return fmt.Errorf("缺少 openid，无法完成内容安全检查")
	}
	if len(data) == 0 {
		return nil
	}
	if len(data) > maxImgSecCheckBytes {
		return ErrImageTooLargeForSecCheck
	}
	if s.token == nil {
		return fmt.Errorf("内容安全服务未配置")
	}
	token, err := s.token.GetAccessToken()
	if err != nil {
		return fmt.Errorf("获取 access_token 失败: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	name := filepath.Base(filename)
	if name == "" || name == "." {
		name = "image.jpg"
	}
	part, err := writer.CreateFormFile("media", name)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/wxa/img_sec_check?access_token=%s", secCheckAPIBase, token)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("图片内容安全检查请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("解析图片内容安全响应失败: %w", err)
	}
	if result.ErrCode == 87014 {
		return ErrContentUnsafe
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("图片内容安全检查失败: [%d] %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

// CheckFridgeInputs 检测冰箱食材名称与备注。
func (s *SecCheckService) CheckFridgeInputs(openid string, inputs []FridgeItemInput) error {
	for _, in := range inputs {
		if err := s.CheckTexts(openid, SecCheckSceneProfile, in.Name, in.Note); err != nil {
			return err
		}
	}
	return nil
}

func (s *SecCheckService) checkOne(openid string, scene int, content string) error {
	if s.token == nil {
		return fmt.Errorf("内容安全服务未配置")
	}
	token, err := s.token.GetAccessToken()
	if err != nil {
		return fmt.Errorf("获取 access_token 失败: %w", err)
	}
	body, _ := json.Marshal(map[string]any{
		"version": 2,
		"openid":  openid,
		"scene":   scene,
		"content": content,
	})
	url := fmt.Sprintf("%s/wxa/msg_sec_check?access_token=%s", secCheckAPIBase, token)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("内容安全检查请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			Suggest string `json:"suggest"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("解析内容安全响应失败: %w", err)
	}
	if result.ErrCode == 87014 {
		return ErrContentUnsafe
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("内容安全检查失败: [%d] %s", result.ErrCode, result.ErrMsg)
	}
	switch strings.ToLower(result.Result.Suggest) {
	case "pass", "":
		return nil
	default:
		return ErrContentUnsafe
	}
}

func extractStringsFromJSON(fields ...string) []string {
	out := make([]string, 0, 8)
	for _, raw := range fields {
		raw = strings.TrimSpace(raw)
		if raw == "" || raw == "[]" || raw == "null" {
			continue
		}
		var strs []string
		if err := json.Unmarshal([]byte(raw), &strs); err == nil {
			out = append(out, strs...)
			continue
		}
		var objs []map[string]string
		if err := json.Unmarshal([]byte(raw), &objs); err == nil {
			for _, o := range objs {
				for _, k := range []string{"name", "amount", "text", "step", "content", "value"} {
					if v := strings.TrimSpace(o[k]); v != "" {
						out = append(out, v)
					}
				}
			}
			continue
		}
		out = append(out, raw)
	}
	return out
}
