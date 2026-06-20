package notifier_test
import (
	"recipe-server/internal/service/notifier"
	"context"
	"testing"

	"recipe-server/config"
)

type stubTokenProvider struct {
	token string
	err   error
}

func (s stubTokenProvider) GetAccessToken() (string, error) {
	return s.token, s.err
}

func TestWeChatSubscribeMissingOpenID(t *testing.T) {
	n := notifier.NewWeChatSubscribeNotifier(true, stubTokenProvider{token: "tok"})
	_, err := n.Send(context.Background(), notifier.NotificationMessage{}, notifier.NotificationTarget{})
	if err == nil {
		t.Fatal("缺少 openid 应返回错误")
	}
}

func TestWeChatSubscribeUsesMessageOpenID(t *testing.T) {
	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		WeChat: config.WeChatConfig{AppID: "wx", Secret: "sec", TemplateID: "tmpl"},
		Notification: config.NotificationConfig{
			WeChatSubscribe: config.NotificationWxSub{Enabled: true, TemplateID: "tmpl"},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	n := notifier.NewWeChatSubscribeNotifier(false, stubTokenProvider{token: "tok"})
	if n.Enabled() {
		t.Fatal("通道 disabled 时不应启用")
	}
}

func TestWeChatSubscribeDisabledWhenNotConfigured(t *testing.T) {
	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	n := notifier.NewWeChatSubscribeNotifier(true, stubTokenProvider{token: "tok"})
	if n.Enabled() {
		t.Fatal("未配置微信凭据时不应启用")
	}
}

func TestWeChatSubscribeChannelName(t *testing.T) {
	n := notifier.NewWeChatSubscribeNotifier(true, stubTokenProvider{})
	if n.Channel() != "wechat_subscribe" {
		t.Fatalf("channel: %q", n.Channel())
	}
}
