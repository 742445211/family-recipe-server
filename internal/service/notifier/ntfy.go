package notifier

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"recipe-server/config"
)

// NtfyNotifier ntfy 推送。
type NtfyNotifier struct {
	enabled bool
	client  *http.Client
}

func NewNtfyNotifier(enabled bool) *NtfyNotifier {
	return &NtfyNotifier{enabled: enabled, client: &http.Client{}}
}

func (n *NtfyNotifier) Channel() string { return "ntfy" }
func (n *NtfyNotifier) Enabled() bool   { return n.enabled }

func (n *NtfyNotifier) Send(ctx context.Context, msg NotificationMessage, target NotificationTarget) (*SendResult, error) {
	_ = ctx
	topic := target.Topic
	if topic == "" {
		return &SendResult{Status: "failed", ErrorCode: "NO_TOPIC", ErrorMessage: "缺少 topic", Retryable: false}, fmt.Errorf("missing topic")
	}
	endpoint := target.Endpoint
	if endpoint == "" {
		endpoint = config.AppConfig.Notification.Ntfy.DefaultEndpoint
	}
	endpoint = trimSlash(endpoint)
	apiURL := endpoint + "/" + topic

	req, err := http.NewRequest(http.MethodPost, apiURL, bytesReader([]byte(msg.Content)))
	if err != nil {
		return nil, err
	}
	cfg := config.AppConfig.Notification.Ntfy
	req.Header.Set("Title", msg.Title)
	req.Header.Set("Priority", cfg.DefaultPriority)
	req.Header.Set("Tags", cfg.DefaultTags)
	if target.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+target.Secret)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return &SendResult{Status: "failed", ErrorCode: "NETWORK", ErrorMessage: err.Error(), Retryable: true, MaskedTarget: mask(topic)}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retryable := resp.StatusCode >= 500
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			retryable = false
		}
		return &SendResult{Status: "failed", ErrorCode: fmt.Sprintf("HTTP_%d", resp.StatusCode), ErrorMessage: string(respBody), Retryable: retryable, MaskedTarget: mask(topic)},
			fmt.Errorf("ntfy http %d", resp.StatusCode)
	}
	if strings.Contains(string(respBody), "error") && resp.StatusCode != 200 {
		return &SendResult{Status: "failed", ErrorCode: "NTFY", ErrorMessage: string(respBody), Retryable: false, MaskedTarget: mask(topic)},
			fmt.Errorf("ntfy error")
	}
	return &SendResult{Status: "sent", MaskedTarget: mask(topic)}, nil
}
