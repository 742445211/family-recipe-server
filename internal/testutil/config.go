package testutil

import (
	"testing"

	"recipe-server/config"
)

// EnsureAppConfig 保证测试环境有可用的配置。
// 优先加载项目根目录 config.yaml（能读到则使用其中的值），仅对缺失字段补测试默认值。
func EnsureAppConfig() {
	if config.AppConfig == nil {
		LoadConfigIfPresent()
	}
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
	}
	applyTestConfigDefaults()
}

func applyTestConfigDefaults() {
	if config.AppConfig.JWT.Secret == "" {
		config.AppConfig.JWT.Secret = "test-secret"
	}
	if config.AppConfig.JWT.ExpireHours == 0 {
		config.AppConfig.JWT.ExpireHours = 24
	}
}

// InitTestConfig 测试默认开启通知（保留 config.yaml 中已有配置，仅补缺失项）。
func InitTestConfig() {
	InitTestConfigNotification(true)
}

// InitTestConfigNotification 按指定开关初始化通知测试配置。
func InitTestConfigNotification(enabled bool) {
	EnsureAppConfig()
	applyTestNotificationConfig(enabled)
}

func applyTestNotificationConfig(enabled bool) {
	n := &config.AppConfig.Notification
	n.Enabled = enabled
	n.Worker.Enabled = false
	if !enabled {
		return
	}
	if !n.WebSocket.Enabled {
		n.WebSocket.Enabled = true
	}
	if n.WeChatSubscribe.TemplateID == "" {
		n.WeChatSubscribe.Enabled = true
		n.WeChatSubscribe.TemplateID = "test-template"
	}
	if config.AppConfig.WeChat.AppID == "" {
		config.AppConfig.WeChat.AppID = "test"
	}
	if config.AppConfig.WeChat.Secret == "" {
		config.AppConfig.WeChat.Secret = "test"
	}
}

// RequireNotificationEnabled 通知未开启时跳过用例。
func RequireNotificationEnabled(t *testing.T) {
	t.Helper()
	if config.AppConfig == nil || !config.AppConfig.Notification.Enabled {
		t.Skip("notification.enabled=false，跳过需通知开启的用例")
	}
}
