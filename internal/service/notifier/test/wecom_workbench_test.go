package notifier_test
import (
	"recipe-server/internal/service/notifier"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"recipe-server/config"
)

type fakeTokenProvider struct{ tok string }

func (f fakeTokenProvider) GetAccessToken() (string, error) { return f.tok, nil }

func TestWecomWorkbenchMissingUserid(t *testing.T) {
	n := notifier.NewWecomWorkbenchNotifier(true, nil)
	_, err := n.Send(context.Background(), notifier.NotificationMessage{Title: "t", Content: "c"}, notifier.NotificationTarget{})
	if err == nil {
		t.Fatal("缺少 userid 应返回错误")
	}
}

func TestBuildOrderCardDescription(t *testing.T) {
	msg := notifier.NotificationMessage{
		RecipeName:  "红烧肉",
		AdderName:   "张三",
		MealType:    "dinner",
		Date:        "2026-06-05",
		Ingredients: `[{"name":"五花肉","amount":"500g"}]`,
		Note:        "少油",
	}
	desc := notifier.BuildOrderCardDescription(msg)
	for _, want := range []string{"2026-06-05", "晚餐", "红烧肉", "张三", "五花肉500g", "少油", "<div"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("卡片描述应包含 %q: %s", want, desc)
		}
	}
}

func TestWecomWorkbenchSendsNewsWithRecipeCover(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				Enabled: true, CorpID: "c", AgentID: 7, Secret: "s",
				APIBase: srv.URL, MsgType: "news", CardURL: "https://example.com/order",
				DuplicateCheckInterval: 1800,
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	n := notifier.NewWecomWorkbenchNotifier(true, fakeTokenProvider{tok: "tok"})
	res, err := n.Send(context.Background(),
		notifier.NotificationMessage{
			Title: "有新的点菜", RecipeName: "红烧肉", AdderName: "张三",
			MealType: "dinner", Date: "2026-06-05",
			RecipeCoverURL: "https://cdn.example.com/hongshaorou.jpg",
		},
		notifier.NotificationTarget{Secret: "useridA"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if res.Status != "sent" {
		t.Fatalf("status: %+v", res)
	}
	if payload["msgtype"] != "news" {
		t.Fatalf("有封面图时应发送 news 图文卡片, got %v", payload["msgtype"])
	}
	news, ok := payload["news"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 news 字段: %v", payload)
	}
	articles, ok := news["articles"].([]any)
	if !ok || len(articles) == 0 {
		t.Fatalf("news.articles 应非空: %v", news)
	}
	article, ok := articles[0].(map[string]any)
	if !ok {
		t.Fatalf("article 格式错误: %v", articles[0])
	}
	if article["picurl"] != "https://cdn.example.com/hongshaorou.jpg" {
		t.Fatalf("顶部图片应使用菜品封面 picurl=%v", article["picurl"])
	}
	if article["url"] != "https://example.com/order" {
		t.Fatalf("news.url 应取配置 card_url: %v", article["url"])
	}
	desc, _ := article["description"].(string)
	if !strings.Contains(desc, "红烧肉") || !strings.Contains(desc, "张三") {
		t.Fatalf("news.description 应包含菜名与点菜人: %v", desc)
	}
}

func TestWecomWorkbenchSendsNewsWithMiniProgramJump(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				Enabled: true, CorpID: "c", AgentID: 7, Secret: "s",
				APIBase: srv.URL, MsgType: "news", CardURL: "https://example.com/order",
				MiniAppID: "wx-mini", MiniPagepath: "pages/order/order",
				DuplicateCheckInterval: 1800,
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	n := notifier.NewWecomWorkbenchNotifier(true, fakeTokenProvider{tok: "tok"})
	_, err := n.Send(context.Background(),
		notifier.NotificationMessage{
			Title: "有新的点菜", RecipeName: "红烧肉", AdderName: "张三",
			MealType: "dinner", Date: "2026-06-05",
			RecipeCoverURL: "https://cdn.example.com/hongshaorou.jpg",
		},
		notifier.NotificationTarget{Secret: "useridA"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	article := firstNewsArticle(t, payload)
	if article["appid"] != "wx-mini" {
		t.Fatalf("应发送 appid: %v", article["appid"])
	}
	if article["pagepath"] != "pages/order/order" {
		t.Fatalf("应发送 pagepath: %v", article["pagepath"])
	}
	if _, hasURL := article["url"]; hasURL {
		t.Fatalf("配置小程序跳转时不应发送 url: %v", article["url"])
	}
}

func TestWecomWorkbenchMiniAppIDFallsBackToWechatAppID(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		WeChat: config.WeChatConfig{AppID: "wx-from-wechat"},
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				Enabled: true, CorpID: "c", AgentID: 7, Secret: "s",
				APIBase: srv.URL, MsgType: "news",
				MiniPagepath: "pages/order/order", DuplicateCheckInterval: 1800,
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	n := notifier.NewWecomWorkbenchNotifier(true, fakeTokenProvider{tok: "tok"})
	_, err := n.Send(context.Background(),
		notifier.NotificationMessage{
			RecipeName: "红烧肉", AdderName: "张三", MealType: "dinner", Date: "2026-06-05",
			RecipeCoverURL: "https://cdn.example.com/cover.jpg",
		},
		notifier.NotificationTarget{Secret: "useridA"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	article := firstNewsArticle(t, payload)
	if article["appid"] != "wx-from-wechat" {
		t.Fatalf("未配置 mini_appid 时应回退 wechat.appid: %v", article["appid"])
	}
}

func TestWecomWorkbenchSendsNewsWithMiniJumpWithoutCover(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		WeChat: config.WeChatConfig{AppID: "wx123"},
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				Enabled: true, CorpID: "c", AgentID: 7, Secret: "s",
				APIBase: srv.URL, MsgType: "news",
				MiniPagepath: "pages/order/order", DuplicateCheckInterval: 1800,
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	n := notifier.NewWecomWorkbenchNotifier(true, fakeTokenProvider{tok: "tok"})
	_, err := n.Send(context.Background(),
		notifier.NotificationMessage{RecipeName: "红烧肉", AdderName: "张三", MealType: "dinner", Date: "2026-06-05"},
		notifier.NotificationTarget{Secret: "useridA"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if payload["msgtype"] != "news" {
		t.Fatalf("配置小程序跳转时无封面也应发 news, got %v", payload["msgtype"])
	}
	article := firstNewsArticle(t, payload)
	if article["appid"] != "wx123" || article["pagepath"] != "pages/order/order" {
		t.Fatalf("应带小程序跳转字段: %v", article)
	}
}

func TestWecomWorkbenchTextCardIgnoresMiniJump(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		WeChat: config.WeChatConfig{AppID: "wx123"},
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				Enabled: true, CorpID: "c", AgentID: 7, Secret: "s",
				APIBase: srv.URL, MsgType: "textcard", CardURL: "https://example.com",
				MiniPagepath: "pages/order/order", DuplicateCheckInterval: 1800,
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	n := notifier.NewWecomWorkbenchNotifier(true, fakeTokenProvider{tok: "tok"})
	_, err := n.Send(context.Background(),
		notifier.NotificationMessage{RecipeName: "红烧肉", AdderName: "张三", MealType: "dinner", Date: "2026-06-05"},
		notifier.NotificationTarget{Secret: "useridA"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if payload["msgtype"] != "textcard" {
		t.Fatalf("无封面 textcard 模式应保持 textcard, got %v", payload["msgtype"])
	}
	card, ok := payload["textcard"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 textcard: %v", payload)
	}
	if card["url"] != "https://example.com" {
		t.Fatalf("textcard 应使用 card_url: %v", card["url"])
	}
	if _, hasAppid := card["appid"]; hasAppid {
		t.Fatal("textcard 不支持小程序跳转，不应带 appid")
	}
}

func firstNewsArticle(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	news, ok := payload["news"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 news 字段: %v", payload)
	}
	articles, ok := news["articles"].([]any)
	if !ok || len(articles) == 0 {
		t.Fatalf("news.articles 应非空: %v", news)
	}
	article, ok := articles[0].(map[string]any)
	if !ok {
		t.Fatalf("article 格式错误: %v", articles[0])
	}
	return article
}

func TestWecomWorkbenchSendsTextCardWithoutCover(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				Enabled: true, CorpID: "c", AgentID: 7, Secret: "s",
				APIBase: srv.URL, MsgType: "textcard", CardURL: "https://example.com",
				DuplicateCheckInterval: 1800,
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	n := notifier.NewWecomWorkbenchNotifier(true, fakeTokenProvider{tok: "tok"})
	_, err := n.Send(context.Background(),
		notifier.NotificationMessage{RecipeName: "红烧肉", AdderName: "张三", MealType: "dinner", Date: "2026-06-05"},
		notifier.NotificationTarget{Secret: "useridA"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if payload["msgtype"] != "textcard" {
		t.Fatalf("无封面图时应发送 textcard, got %v", payload["msgtype"])
	}
	card, ok := payload["textcard"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 textcard 字段: %v", payload)
	}
	desc, _ := card["description"].(string)
	if !strings.Contains(desc, "红烧肉") || !strings.Contains(desc, "张三") {
		t.Fatalf("textcard.description 应包含菜名与点菜人: %v", desc)
	}
}

func TestRecipeCoverURL(t *testing.T) {
	msg := notifier.NotificationMessage{RecipeCoverURL: "https://a.com/1.jpg"}
	cfg := config.NotificationWecom{DefaultCoverURL: "https://default.jpg"}
	if got := notifier.RecipeCoverURLForTest(msg, cfg); got != "https://a.com/1.jpg" {
		t.Fatalf("应优先使用菜品封面: %q", got)
	}
	msg.RecipeCoverURL = ""
	if got := notifier.RecipeCoverURLForTest(msg, cfg); got != "https://default.jpg" {
		t.Fatalf("无菜品封面时应回落默认图: %q", got)
	}
}

func TestWecomWorkbenchSendsText(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				Enabled: true, CorpID: "c", AgentID: 7, Secret: "s",
				APIBase: srv.URL, MsgType: "text", DuplicateCheckInterval: 1800,
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	n := notifier.NewWecomWorkbenchNotifier(true, fakeTokenProvider{tok: "tok"})
	_, err := n.Send(context.Background(),
		notifier.NotificationMessage{RecipeName: "红烧肉", AdderName: "张三", MealType: "dinner", Date: "2026-06-05", Content: "正文内容"},
		notifier.NotificationTarget{Secret: "useridA"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if payload["msgtype"] != "text" {
		t.Fatalf("应发送 text, got %v", payload["msgtype"])
	}
	text, _ := payload["text"].(map[string]any)
	if text["content"] != "正文内容" {
		t.Fatalf("text.content 应使用 msg.Content: %v", text)
	}
}

func TestBuildOrderContent(t *testing.T) {
	msg := notifier.NotificationMessage{
		RecipeName:  "红烧肉",
		AdderName:   "张三",
		MealType:    "dinner",
		Date:        "2026-06-05",
		Ingredients: `[{"name":"番茄","amount":"2个"},{"name":"鸡蛋","amount":"3个"}]`,
		Note:        "少油",
	}
	content := notifier.BuildOrderContent(msg)
	if content == "" {
		t.Fatal("content 不应为空")
	}
	if notifier.MealName("dinner") != "晚餐" {
		t.Fatal("餐次映射错误")
	}
	for _, want := range []string{"2026-06-05", "晚餐", "红烧肉", "张三", "食材：", "番茄2个", "鸡蛋3个", "备注：少油"} {
		if !strings.Contains(content, want) {
			t.Fatalf("content 应包含 %q，实际: %s", want, content)
		}
	}
}

func TestFormatIngredients(t *testing.T) {
	got := notifier.FormatIngredients(`[{"name":"番茄","amount":"2个"},{"name":"鸡蛋","amount":"3个"}]`)
	if got != "番茄2个、鸡蛋3个" {
		t.Fatalf("notifier.FormatIngredients: got %q", got)
	}
	if notifier.FormatIngredients("") != "" {
		t.Fatal("空食材应返回空字符串")
	}
}
