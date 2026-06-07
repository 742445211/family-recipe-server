package wechattoken

import (
	"encoding/json"
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

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
