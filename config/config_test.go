package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMySQLConfigDSN(t *testing.T) {
	dsn := MySQLConfig{
		User:     "root",
		Password: "secret",
		Host:     "127.0.0.1",
		Port:     3306,
		Database: "recipe_app",
	}.DSN()
	want := "root:secret@tcp(127.0.0.1:3306)/recipe_app?charset=utf8mb4&parseTime=True&loc=Local"
	if dsn != want {
		t.Fatalf("DSN: got %q want %q", dsn, want)
	}
}

func TestWeChatConfigured(t *testing.T) {
	if (&Config{}).WeChatConfigured() {
		t.Fatal("空配置不应视为已配置微信")
	}
	cfg := &Config{WeChat: WeChatConfig{AppID: "wx", Secret: "sec"}}
	if !cfg.WeChatConfigured() {
		t.Fatal("AppID 与 Secret 齐全时应返回 true")
	}
}

func TestEffectiveTemplateID(t *testing.T) {
	cfg := &Config{
		WeChat: WeChatConfig{TemplateID: "legacy-tmpl"},
		Notification: NotificationConfig{
			WeChatSubscribe: NotificationWxSub{TemplateID: "new-tmpl"},
		},
	}
	if got := cfg.EffectiveTemplateID(); got != "new-tmpl" {
		t.Fatalf("notification 块优先: got %q", got)
	}
	cfg.Notification.WeChatSubscribe.TemplateID = ""
	if got := cfg.EffectiveTemplateID(); got != "legacy-tmpl" {
		t.Fatalf("应回退 wechat.template_id: got %q", got)
	}
}

func TestEffectiveMiniprogramState(t *testing.T) {
	cfg := &Config{
		WeChat: WeChatConfig{MiniprogramState: "formal"},
		Notification: NotificationConfig{
			WeChatSubscribe: NotificationWxSub{MiniprogramState: "trial"},
		},
	}
	if got := cfg.EffectiveMiniprogramState(); got != "trial" {
		t.Fatalf("notification 块优先: got %q", got)
	}
	cfg.Notification.WeChatSubscribe.MiniprogramState = ""
	if got := cfg.EffectiveMiniprogramState(); got != "formal" {
		t.Fatalf("应回退 wechat.miniprogram_state: got %q", got)
	}
	cfg.WeChat.MiniprogramState = ""
	if got := cfg.EffectiveMiniprogramState(); got != "developer" {
		t.Fatalf("默认 developer: got %q", got)
	}
}

func TestEffectiveWecomMiniAppID(t *testing.T) {
	cfg := &Config{
		WeChat: WeChatConfig{AppID: "wx-fallback"},
		Notification: NotificationConfig{
			WecomWorkbench: NotificationWecom{MiniAppID: "wx-explicit"},
		},
	}
	if got := cfg.EffectiveWecomMiniAppID(); got != "wx-explicit" {
		t.Fatalf("wecom_workbench.mini_appid 优先: got %q", got)
	}
	cfg.Notification.WecomWorkbench.MiniAppID = ""
	if got := cfg.EffectiveWecomMiniAppID(); got != "wx-fallback" {
		t.Fatalf("应回退 wechat.appid: got %q", got)
	}
}

func TestWecomMiniProgramJumpConfigured(t *testing.T) {
	cfg := &Config{
		WeChat: WeChatConfig{AppID: "wx123"},
		Notification: NotificationConfig{
			WecomWorkbench: NotificationWecom{MiniPagepath: "pages/order/order"},
		},
	}
	if !cfg.WecomMiniProgramJumpConfigured() {
		t.Fatal("appid 与 pagepath 齐全时应返回 true")
	}
	cfg.Notification.WecomWorkbench.MiniPagepath = ""
	if cfg.WecomMiniProgramJumpConfigured() {
		t.Fatal("缺少 pagepath 时应返回 false")
	}
	cfg.Notification.WecomWorkbench.MiniPagepath = "pages/order/order"
	cfg.WeChat.AppID = ""
	if cfg.WecomMiniProgramJumpConfigured() {
		t.Fatal("缺少 appid 时应返回 false")
	}
}

func TestWeChatSubscribeConfigured(t *testing.T) {
	cfg := &Config{
		WeChat: WeChatConfig{AppID: "wx", Secret: "sec", TemplateID: "tmpl"},
		Notification: NotificationConfig{
			WeChatSubscribe: NotificationWxSub{Enabled: true},
		},
	}
	if !cfg.WeChatSubscribeConfigured() {
		t.Fatal("凭据与模板齐全且通道开启时应返回 true")
	}
	cfg.Notification.WeChatSubscribe.Enabled = false
	if cfg.WeChatSubscribeConfigured() {
		t.Fatal("通道关闭时应返回 false")
	}
}

func TestAIRecommendEnabled(t *testing.T) {
	if (&Config{}).AIRecommendEnabled() {
		t.Fatal("缺省 recommend_enabled 应为 false")
	}
	cfg := &Config{AI: AIConfig{RecommendEnabled: true}}
	if !cfg.AIRecommendEnabled() {
		t.Fatal("recommend_enabled=true 时应返回 true")
	}
	cfg.AI.RecommendEnabled = false
	if cfg.AIRecommendEnabled() {
		t.Fatal("recommend_enabled=false 时应返回 false")
	}
	if (*Config)(nil).AIRecommendEnabled() {
		t.Fatal("nil Config 应返回 false")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
server: {}
mysql: {host: db.local, port: 3307, user: u, password: p, database: d}
jwt: {secret: s, expire_hours: 1}
wechat: {appid: wx, secret: sec}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	old := AppConfig
	t.Cleanup(func() { AppConfig = old })

	if err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if AppConfig.Server.Port != 8080 {
		t.Fatalf("默认端口 8080, got %d", AppConfig.Server.Port)
	}
	if AppConfig.Server.Mode != "debug" {
		t.Fatalf("默认 mode debug, got %q", AppConfig.Server.Mode)
	}
	if AppConfig.Notification.WebSocket.Path != "/api/ws" {
		t.Fatalf("默认 ws path: got %q", AppConfig.Notification.WebSocket.Path)
	}
	if AppConfig.Weather.DefaultCity != "成都" {
		t.Fatalf("默认城市成都, got %q", AppConfig.Weather.DefaultCity)
	}
	if AppConfig.AI.RateLimit.Recommend.MaxRequests != 5 {
		t.Fatalf("默认 recommend 限流 5 次, got %d", AppConfig.AI.RateLimit.Recommend.MaxRequests)
	}
	if AppConfig.AI.RateLimit.Catalog.WindowHours != 2 {
		t.Fatalf("默认 catalog 窗口 2h, got %d", AppConfig.AI.RateLimit.Catalog.WindowHours)
	}
}

func TestCatalogRecipeEnabled(t *testing.T) {
	cfg := &Config{AI: AIConfig{RecommendEnabled: true}}
	if !cfg.CatalogRecipeEnabled() {
		t.Fatal("recommend 开启时 catalog 默认应开启")
	}
	disabled := false
	cfg.AI.CatalogEnabled = &disabled
	if cfg.CatalogRecipeEnabled() {
		t.Fatal("catalog_enabled=false 时应关闭")
	}
}
