package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"recipe-server/config"
)

var (
	accessToken   string
	tokenExpireAt time.Time
	tokenMu       sync.Mutex
)

// GetAccessToken 获取微信公众号/小程序 access_token（带缓存）
func GetAccessToken() (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()

	if accessToken != "" && time.Now().Before(tokenExpireAt) {
		return accessToken, nil
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		config.AppConfig.WeChat.AppID, config.AppConfig.WeChat.Secret)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("get access_token: %w", err)
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
		return "", fmt.Errorf("parse access_token: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("get access_token error: %s", result.ErrMsg)
	}

	accessToken = result.AccessToken
	// 提前5分钟过期，安全余量
	tokenExpireAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	return accessToken, nil
}

type templateData map[string]struct {
	Value string `json:"value"`
}

// SendOrderNotify 发送点菜通知给厨师
func SendOrderNotify(openid, recipeName, adderName, mealType, date string) error {
	if config.AppConfig.WeChat.TemplateID == "" {
		return nil // 未配置模板ID，跳过
	}

	token, err := GetAccessToken()
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	mealMap := map[string]string{"breakfast": "早餐", "lunch": "午餐", "dinner": "晚餐"}
	mealName := mealMap[mealType]
	if mealName == "" {
		mealName = mealType
	}

	data := templateData{
		"thing1":  {Value: truncate(recipeName, 20)},  // 菜名
		"name2":   {Value: truncate(adderName, 20)},    // 点菜人
		"thing3":  {Value: mealName},                    // 餐次
		"date4":   {Value: date},                        // 日期
		"thing5":  {Value: "有人点了新菜，快去看看吧~"},     // 备注
	}

	body, _ := json.Marshal(map[string]any{
		"touser":            openid,
		"template_id":       config.AppConfig.WeChat.TemplateID,
		"page":              "pages/order/order",
		"miniprogram_state": "formal",
		"lang":              "zh_CN",
		"data":              data,
	})

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=%s", token)
	resp, err := http.Post(url, "application/json", bytesReader(body))
	if err != nil {
		return fmt.Errorf("send notify: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.Unmarshal(respBody, &result)
	if result.ErrCode != 0 {
		return fmt.Errorf("send notify error [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func bytesReader(b []byte) *bytesReadCloser {
	return &bytesReadCloser{b: b, pos: 0}
}

type bytesReadCloser struct {
	b   []byte
	pos int
}

func (r *bytesReadCloser) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.b) {
		return 0, io.EOF
	}
	n = copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}

func (r *bytesReadCloser) Close() error { return nil }
