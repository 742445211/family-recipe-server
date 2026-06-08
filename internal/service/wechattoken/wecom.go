package wechattoken

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"recipe-server/config"
)

// WecomToken 企业微信 access_token 缓存。
type WecomToken struct {
	mu      sync.Mutex
	token   string
	expires time.Time
	client  *http.Client
}

// NewWecomToken 创建企微 token 提供者。
func NewWecomToken() *WecomToken {
	return &WecomToken{client: &http.Client{}}
}

func (w *WecomToken) GetAccessToken() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.token != "" && time.Now().Before(w.expires) {
		return w.token, nil
	}
	cfg := config.AppConfig.Notification.WecomWorkbench
	url := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		trimSlash(cfg.APIBase), cfg.CorpID, cfg.Secret)
	resp, err := w.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("%s", result.ErrMsg)
	}
	w.token = result.AccessToken
	w.expires = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	return w.token, nil
}

// GetUseridByMobile 通过成员手机号查询企业微信 UserID（user/getuserid 接口）。
func (w *WecomToken) GetUseridByMobile(mobile string) (string, error) {
	token, err := w.GetAccessToken()
	if err != nil {
		return "", err
	}
	cfg := config.AppConfig.Notification.WecomWorkbench
	url := fmt.Sprintf("%s/cgi-bin/user/getuserid?access_token=%s", trimSlash(cfg.APIBase), token)
	body, _ := json.Marshal(map[string]string{"mobile": mobile})
	resp, err := w.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		UserID  string `json:"userid"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("企业微信查询手机号失败: %s", result.ErrMsg)
	}
	if result.UserID == "" {
		return "", errors.New("企业微信未找到该手机号对应成员")
	}
	return result.UserID, nil
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
