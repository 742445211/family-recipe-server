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

// Provider access_token 提供者。
type Provider interface {
	GetAccessToken() (string, error)
}

// MiniProgramToken 小程序 access_token 缓存。
type MiniProgramToken struct {
	mu      sync.Mutex
	token   string
	expires time.Time
	client  *http.Client
}

// NewMiniProgramToken 创建小程序 token 提供者。
func NewMiniProgramToken() *MiniProgramToken {
	return &MiniProgramToken{client: &http.Client{}}
}

var sharedMiniProgram = NewMiniProgramToken()

// SharedMiniProgramToken 进程内唯一小程序 token 缓存（避免多处各自刷新导致 40001）。
func SharedMiniProgramToken() Provider {
	return sharedMiniProgram
}

// InvalidateSharedMiniProgramToken 清除共享 token，供 40001 后强制刷新。
func InvalidateSharedMiniProgramToken() {
	sharedMiniProgram.Invalidate()
}

func (m *MiniProgramToken) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = ""
	m.expires = time.Time{}
}

func (m *MiniProgramToken) GetAccessToken() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != "" && time.Now().Before(m.expires) {
		return m.token, nil
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		config.AppConfig.WeChat.AppID, config.AppConfig.WeChat.Secret)
	resp, err := m.client.Get(url)
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
	m.token = result.AccessToken
	m.expires = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	return m.token, nil
}
