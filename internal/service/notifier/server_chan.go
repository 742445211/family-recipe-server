package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"recipe-server/config"
)

// ServerChanNotifier Server酱推送。
type ServerChanNotifier struct {
	enabled bool
	client  *http.Client
}

func NewServerChanNotifier(enabled bool) *ServerChanNotifier {
	return &ServerChanNotifier{enabled: enabled, client: &http.Client{}}
}

func (n *ServerChanNotifier) Channel() string { return "server_chan" }
func (n *ServerChanNotifier) Enabled() bool   { return n.enabled }

func (n *ServerChanNotifier) Send(ctx context.Context, msg NotificationMessage, target NotificationTarget) (*SendResult, error) {
	_ = ctx
	sendKey := target.Secret
	if sendKey == "" {
		return &SendResult{Status: "failed", ErrorCode: "NO_SENDKEY", ErrorMessage: "缺少 SendKey", Retryable: false}, fmt.Errorf("missing sendkey")
	}

	base := trimSlash(config.AppConfig.Notification.ServerChan.APIBase)
	apiURL := fmt.Sprintf("%s/%s.send", base, sendKey)
	desp := BuildOrderContent(msg)
	form := url.Values{}
	form.Set("title", msg.Title)
	form.Set("desp", desp)
	form.Set("short", truncateRunes(msg.RecipeName, 64))

	resp, err := n.client.PostForm(apiURL, form)
	if err != nil {
		return &SendResult{Status: "failed", ErrorCode: "NETWORK", ErrorMessage: err.Error(), Retryable: true, MaskedTarget: mask(sendKey)}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 500 {
		return &SendResult{Status: "failed", ErrorCode: "HTTP_5XX", ErrorMessage: string(respBody), Retryable: true, MaskedTarget: mask(sendKey)},
			fmt.Errorf("serverchan 5xx")
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
	}
	_ = json.Unmarshal(respBody, &result)
	if result.Code != 0 && !strings.Contains(string(respBody), "success") {
		retryable := resp.StatusCode >= 500
		return &SendResult{Status: "failed", ErrorCode: fmt.Sprintf("%d", result.Code), ErrorMessage: result.Msg, Retryable: retryable, MaskedTarget: mask(sendKey)},
			fmt.Errorf("serverchan error: %s", result.Msg)
	}
	return &SendResult{Status: "sent", MaskedTarget: mask(sendKey)}, nil
}
