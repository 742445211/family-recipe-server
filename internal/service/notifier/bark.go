package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"recipe-server/config"
)

// BarkNotifier Bark iOS 推送。
type BarkNotifier struct {
	enabled bool
	client  *http.Client
}

func NewBarkNotifier(enabled bool) *BarkNotifier {
	return &BarkNotifier{enabled: enabled, client: &http.Client{}}
}

func (n *BarkNotifier) Channel() string { return "bark" }
func (n *BarkNotifier) Enabled() bool   { return n.enabled }

func (n *BarkNotifier) Send(ctx context.Context, msg NotificationMessage, target NotificationTarget) (*SendResult, error) {
	_ = ctx
	deviceKey := target.Secret
	if deviceKey == "" {
		return &SendResult{Status: "failed", ErrorCode: "NO_DEVICE_KEY", ErrorMessage: "缺少 Device Key", Retryable: false}, fmt.Errorf("missing device key")
	}
	endpoint := target.Endpoint
	if endpoint == "" {
		endpoint = config.AppConfig.Notification.Bark.DefaultEndpoint
	}
	endpoint = trimSlash(endpoint)

	body, _ := json.Marshal(map[string]any{
		"device_key": deviceKey,
		"title":      msg.Title,
		"body":       msg.Content,
		"group":      "家庭点菜",
		"sound":      "bell",
	})
	apiURL := endpoint + "/push"
	resp, err := n.client.Post(apiURL, "application/json", bytesReader(body))
	if err != nil {
		return &SendResult{Status: "failed", ErrorCode: "NETWORK", ErrorMessage: err.Error(), Retryable: true, MaskedTarget: mask(deviceKey)}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retryable := resp.StatusCode >= 500
		return &SendResult{Status: "failed", ErrorCode: fmt.Sprintf("HTTP_%d", resp.StatusCode), ErrorMessage: string(respBody), Retryable: retryable, MaskedTarget: mask(deviceKey)},
			fmt.Errorf("bark http %d", resp.StatusCode)
	}
	var result struct {
		Code int `json:"code"`
	}
	_ = json.Unmarshal(respBody, &result)
	if result.Code != 200 && !strings.Contains(string(respBody), "success") {
		return &SendResult{Status: "failed", ErrorCode: "BARK", ErrorMessage: string(respBody), Retryable: false, MaskedTarget: mask(deviceKey)},
			fmt.Errorf("bark error")
	}
	return &SendResult{Status: "sent", MaskedTarget: mask(deviceKey)}, nil
}
