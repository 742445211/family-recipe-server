package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"recipe-server/config"
	"recipe-server/internal/service/wechattoken"
	"recipe-server/pkg/dateutil"
)

// WeChatSubscribeNotifier 微信一次性订阅消息。
type WeChatSubscribeNotifier struct {
	enabled bool
	client  *http.Client
	token   wechattoken.Provider
}

// NewWeChatSubscribeNotifier 创建订阅消息通知器。
func NewWeChatSubscribeNotifier(enabled bool, token wechattoken.Provider) *WeChatSubscribeNotifier {
	return &WeChatSubscribeNotifier{
		enabled: enabled,
		client:  &http.Client{},
		token:   token,
	}
}

func (n *WeChatSubscribeNotifier) Channel() string { return "wechat_subscribe" }
func (n *WeChatSubscribeNotifier) Enabled() bool {
	return n.enabled && config.AppConfig != nil && config.AppConfig.WeChatSubscribeConfigured()
}

func (n *WeChatSubscribeNotifier) Send(ctx context.Context, msg NotificationMessage, target NotificationTarget) (*SendResult, error) {
	_ = ctx
	openid := target.OpenID
	if openid == "" {
		openid = msg.OpenID
	}
	if openid == "" {
		return &SendResult{Status: "failed", ErrorCode: "NO_OPENID", ErrorMessage: "缺少 openid", Retryable: false}, fmt.Errorf("missing openid")
	}

	tmpl := config.AppConfig.EffectiveTemplateID()
	token, err := n.token.GetAccessToken()
	if err != nil {
		return &SendResult{Status: "failed", ErrorCode: "TOKEN", ErrorMessage: err.Error(), Retryable: true}, err
	}

	meal := MealName(msg.MealType)
	page := config.AppConfig.Notification.WeChatSubscribe.Page
	if page == "" {
		page = "pages/order/order"
	}
	data := map[string]any{
		"time7":   map[string]string{"value": dateutil.FormatYMD(msg.Date) + " " + meal},
		"thing14": map[string]string{"value": truncateRunes(msg.AdderName, 20)},
		"thing13": map[string]string{"value": truncateRunes(msg.RecipeName, 20)},
	}
	body, _ := json.Marshal(map[string]any{
		"touser":            openid,
		"template_id":       tmpl,
		"page":              page,
		"miniprogram_state": config.AppConfig.EffectiveMiniprogramState(),
		"lang":              "zh_CN",
		"data":              data,
	})
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=%s", token)
	resp, err := n.client.Post(url, "application/json", bytesReader(body))
	if err != nil {
		return &SendResult{Status: "failed", ErrorCode: "NETWORK", ErrorMessage: err.Error(), Retryable: true, MaskedTarget: mask(openid)}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	_ = json.Unmarshal(respBody, &result)
	if result.ErrCode != 0 {
		retryable := result.ErrCode == -1 || result.ErrCode >= 50000
		return &SendResult{
			Status: "failed", ErrorCode: fmt.Sprintf("%d", result.ErrCode),
			ErrorMessage: result.ErrMsg, Retryable: retryable, MaskedTarget: mask(openid),
		}, fmt.Errorf("wechat subscribe error: %s", result.ErrMsg)
	}
	return &SendResult{Status: "sent", MaskedTarget: mask(openid)}, nil
}
