package service

import (
	"encoding/json"
	"fmt"
	"net/http"

	"recipe-server/config"
)

// WeChatSession 微信登录返回
type WeChatSession struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// Code2Session 用 code 换取微信 session
func Code2Session(code string) (*WeChatSession, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		config.AppConfig.WeChat.AppID,
		config.AppConfig.WeChat.Secret,
		code,
	)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("微信请求失败: %w", err)
	}
	defer resp.Body.Close()

	var session WeChatSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("解析微信响应失败: %w", err)
	}
	if session.ErrCode != 0 {
		return nil, fmt.Errorf("微信错误 [%d]: %s", session.ErrCode, session.ErrMsg)
	}
	return &session, nil
}
