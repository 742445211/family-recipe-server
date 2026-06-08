// Package service - 微信认证服务。
//
// 本文件实现微信小程序登录认证功能，通过微信官方接口将小程序端获取的临时 code
// 换取用户的 OpenID、SessionKey 和 UnionID，完成服务端的身份认证流程。
//
// 微信小程序登录流程（参考 https://developers.weixin.qq.com/miniprogram/dev/framework/open-ability/login.html）：
//  1. 小程序端调用 wx.login() 获取临时 code
//  2. 服务端接收 code，调用本文件 Code2Session 换取 session_key 和 openid
//  3. 服务端使用 openid 查找或创建用户，生成 JWT 返回给小程序
package service

import (
	"encoding/json"
	"fmt"
	"net/http"

	"recipe-server/config"
)

// wechatAPIBase 微信登录 API 根地址（测试可替换为 httptest 地址）。
var wechatAPIBase = "https://api.weixin.qq.com"

// WeChatSession 微信登录凭证校验接口返回的会话信息。
// 对应微信 API jscode2session 的响应结构。
// 文档参考：https://developers.weixin.qq.com/miniprogram/dev/OpenApiDoc/user-login/code2Session.html
type WeChatSession struct {
	OpenID     string `json:"openid"`      // 用户唯一标识（在单个小程序内唯一）
	SessionKey string `json:"session_key"` // 会话密钥（用于数据解密，如获取手机号）
	UnionID    string `json:"unionid"`     // 用户在开放平台的唯一标识（需绑定开放平台才有）
	ErrCode    int    `json:"errcode"`     // 错误码，0 表示成功
	ErrMsg     string `json:"errmsg"`      // 错误信息
}

// SetWechatAPIBaseForTest 测试专用：替换微信登录 API 基址。
func SetWechatAPIBaseForTest(base string) {
	wechatAPIBase = base
}

// WechatAPIBaseForTest 测试专用：读取当前微信登录 API 基址。
func WechatAPIBaseForTest() string {
	return wechatAPIBase
}

// Code2Session 用小程序端获取的临时登录凭证 code 换取微信会话信息。
//
// 调用微信官方接口：
//
//	GET https://api.weixin.qq.com/sns/jscode2session?appid=APPID&secret=SECRET&js_code=CODE&grant_type=authorization_code
//
// 参数:
//   - code string - 小程序端调用 wx.login() 获取的临时登录凭证（有效期约 5 分钟）
//
// 返回值:
//   - *WeChatSession - 成功时返回包含 OpenID、SessionKey 等信息的结构体
//   - error           - HTTP 请求失败、JSON 解析失败或微信返回错误码（ErrCode != 0）时返回错误
//
// 注意:
//   - SessionKey 为敏感信息，不应直接返回给前端，仅用于服务端数据解密
//   - UnionID 仅在满足条件时返回（小程序绑定开放平台、用户关注了同主体公众号等）
func Code2Session(code string) (*WeChatSession, error) {
	// 拼接微信 jscode2session 请求 URL（appid 和 secret 来自配置文件）
	url := fmt.Sprintf(
		"%s/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		wechatAPIBase,
		config.AppConfig.WeChat.AppID,
		config.AppConfig.WeChat.Secret,
		code,
	)

	// 发送 HTTP GET 请求
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("微信请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析 JSON 响应体
	var session WeChatSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("解析微信响应失败: %w", err)
	}

	// 检查微信返回的业务错误码（0 表示成功）
	if session.ErrCode != 0 {
		return nil, fmt.Errorf("微信错误 [%d]: %s", session.ErrCode, session.ErrMsg)
	}
	return &session, nil
}
