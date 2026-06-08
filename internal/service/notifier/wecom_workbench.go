package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"recipe-server/config"
	"recipe-server/internal/service/wechattoken"
)

// WecomWorkbenchNotifier 企业微信应用消息（微工作台触达个人微信）。
type WecomWorkbenchNotifier struct {
	enabled bool
	client  *http.Client
	token   wechattoken.Provider
}

// NewWecomWorkbenchNotifier 创建企微通知器。
func NewWecomWorkbenchNotifier(enabled bool, token wechattoken.Provider) *WecomWorkbenchNotifier {
	return &WecomWorkbenchNotifier{
		enabled: enabled,
		client:  &http.Client{},
		token:   token,
	}
}

// cardTitle 卡片标题，优先使用消息标题。
func cardTitle(msg NotificationMessage) string {
	if strings.TrimSpace(msg.Title) != "" {
		return msg.Title
	}
	return "有新的点菜"
}

// wecomCardURL 卡片点击跳转地址。
func wecomCardURL(cfg config.NotificationWecom) string {
	if strings.TrimSpace(cfg.CardURL) != "" {
		return cfg.CardURL
	}
	return "https://www.zzzjc.xin"
}

// recipeCoverURL 解析图文卡片顶部图片：优先菜品封面，其次平台默认封面。
func recipeCoverURL(msg NotificationMessage, cfg config.NotificationWecom) string {
	if u := strings.TrimSpace(msg.RecipeCoverURL); u != "" {
		return u
	}
	return strings.TrimSpace(cfg.DefaultCoverURL)
}

// isWecomCardMode 是否为卡片类消息（news 图文 / textcard 文本卡片）。
func isWecomCardMode(msgType string) bool {
	switch strings.ToLower(strings.TrimSpace(msgType)) {
	case "news", "textcard":
		return true
	default:
		return false
	}
}

// useNewsWithCover 是否使用 news 图文（顶部可展示 picurl 封面图）。
func useNewsWithCover(msgType string, coverURL string) bool {
	if coverURL == "" {
		return false
	}
	// news 模式始终带图；textcard 在有封面时升级为 news 以展示顶部图片。
	switch strings.ToLower(strings.TrimSpace(msgType)) {
	case "news":
		return true
	case "textcard":
		return true
	default:
		return false
	}
}

// wecomMiniJump 返回企微 news 卡片跳转小程序的 appid 与 pagepath。
func wecomMiniJump(cfg config.NotificationWecom) (appid, pagepath string) {
	if config.AppConfig == nil || !config.AppConfig.WecomMiniProgramJumpConfigured() {
		return "", ""
	}
	pagepath = strings.TrimSpace(cfg.MiniPagepath)
	if pagepath == "" {
		return "", ""
	}
	appid = config.AppConfig.EffectiveWecomMiniAppID()
	if appid == "" {
		return "", ""
	}
	return appid, pagepath
}

// useWecomNews 是否发送 news 图文（有封面图，或 msg_type=news 且配置了小程序跳转）。
func useWecomNews(cfg config.NotificationWecom, coverURL string) bool {
	if useNewsWithCover(cfg.MsgType, coverURL) {
		return true
	}
	appid, pagepath := wecomMiniJump(cfg)
	return appid != "" && pagepath != "" && strings.ToLower(strings.TrimSpace(cfg.MsgType)) == "news"
}

// buildNewsArticle 组装企微 news 图文条目。
func buildNewsArticle(cfg config.NotificationWecom, msg NotificationMessage, coverURL string) map[string]string {
	article := map[string]string{
		"title":       cardTitle(msg),
		"description": BuildOrderNewsDescription(msg),
		"btntxt":      "查看详情",
	}
	if coverURL != "" {
		article["picurl"] = coverURL
	}
	if appid, pagepath := wecomMiniJump(cfg); appid != "" {
		article["appid"] = appid
		article["pagepath"] = pagepath
	} else {
		article["url"] = wecomCardURL(cfg)
	}
	return article
}

func (n *WecomWorkbenchNotifier) Channel() string { return "wecom_workbench" }

func (n *WecomWorkbenchNotifier) Enabled() bool {
	cfg := config.AppConfig.Notification.WecomWorkbench
	return n.enabled && cfg.CorpID != "" && cfg.AgentID > 0 && cfg.Secret != ""
}

func (n *WecomWorkbenchNotifier) Send(ctx context.Context, msg NotificationMessage, target NotificationTarget) (*SendResult, error) {
	_ = ctx
	userid := target.Secret
	if userid == "" {
		userid = target.WecomUserid
	}
	if userid == "" {
		return &SendResult{
			Status: "failed", ErrorCode: "NO_USERID",
			ErrorMessage: "缺少企业微信 userid", Retryable: false,
		}, fmt.Errorf("missing wecom userid")
	}

	token, err := n.token.GetAccessToken()
	if err != nil {
		return &SendResult{Status: "failed", ErrorCode: "TOKEN", ErrorMessage: err.Error(), Retryable: true, MaskedTarget: mask(userid)}, err
	}

	cfg := config.AppConfig.Notification.WecomWorkbench
	payload := map[string]any{
		"touser":                   userid,
		"agentid":                  cfg.AgentID,
		"enable_duplicate_check":   1,
		"duplicate_check_interval": cfg.DuplicateCheckInterval,
	}
	n.applyWecomPayload(payload, cfg, msg)

	body, _ := json.Marshal(payload)
	base := trimSlash(cfg.APIBase)
	url := fmt.Sprintf("%s/cgi-bin/message/send?access_token=%s", base, token)
	resp, err := n.client.Post(url, "application/json", bytesReader(body))
	if err != nil {
		return &SendResult{Status: "failed", ErrorCode: "NETWORK", ErrorMessage: err.Error(), Retryable: true, MaskedTarget: mask(userid)}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		InvalidUser string `json:"invaliduser"`
	}
	_ = json.Unmarshal(respBody, &result)
	if result.ErrCode != 0 {
		retryable := result.ErrCode == -1 || strings.Contains(result.ErrMsg, "system")
		if result.InvalidUser != "" || strings.Contains(result.ErrMsg, "invaliduser") {
			retryable = false
		}
		return &SendResult{
			Status: "failed", ErrorCode: fmt.Sprintf("%d", result.ErrCode),
			ErrorMessage: result.ErrMsg, Retryable: retryable, MaskedTarget: mask(userid),
		}, fmt.Errorf("wecom send error: %s", result.ErrMsg)
	}
	return &SendResult{Status: "sent", MaskedTarget: mask(userid)}, nil
}

// applyWecomPayload 按配置与消息内容填充企微消息体。
func (n *WecomWorkbenchNotifier) applyWecomPayload(payload map[string]any, cfg config.NotificationWecom, msg NotificationMessage) {
	cover := recipeCoverURL(msg, cfg)
	switch {
	case useWecomNews(cfg, cover):
		// news 图文：可带 picurl 封面；配置 appid+pagepath 时点击直达关联小程序。
		payload["msgtype"] = "news"
		payload["news"] = map[string]any{
			"articles": []map[string]string{buildNewsArticle(cfg, msg, cover)},
		}
	case isWecomCardMode(cfg.MsgType):
		// textcard 文本卡片：无封面时的兜底样式。
		payload["msgtype"] = "textcard"
		payload["textcard"] = map[string]string{
			"title":       cardTitle(msg),
			"description": BuildOrderCardDescription(msg),
			"url":         wecomCardURL(cfg),
			"btntxt":      "查看详情",
		}
	default:
		// 纯文本。
		content := BuildOrderContent(msg)
		if msg.Content != "" {
			content = msg.Content
		}
		payload["msgtype"] = "text"
		payload["text"] = map[string]string{"content": content}
	}
}
