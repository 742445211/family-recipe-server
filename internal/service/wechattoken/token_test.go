package wechattoken

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"recipe-server/config"
)

func TestMiniProgramTokenCachesUntilExpiry(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"access_token":"tok-abc","expires_in":7200}`))
	}))
	t.Cleanup(srv.Close)

	oldBase := wechatTokenAPIBase
	wechatTokenAPIBase = srv.URL
	t.Cleanup(func() { wechatTokenAPIBase = oldBase })

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{WeChat: config.WeChatConfig{AppID: "wx", Secret: "sec"}}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	m := NewMiniProgramToken()
	tok1, err := m.GetAccessToken()
	if err != nil || tok1 != "tok-abc" {
		t.Fatalf("first: %q err=%v", tok1, err)
	}
	tok2, err := m.GetAccessToken()
	if err != nil || tok2 != "tok-abc" {
		t.Fatalf("cached: %q err=%v", tok2, err)
	}
	if calls != 1 {
		t.Fatalf("应只请求一次微信 API, calls=%d", calls)
	}
}

func TestMiniProgramTokenInvalidate(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"access_token":"tok-new","expires_in":7200}`))
	}))
	t.Cleanup(srv.Close)

	oldBase := wechatTokenAPIBase
	wechatTokenAPIBase = srv.URL
	t.Cleanup(func() { wechatTokenAPIBase = oldBase })

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{WeChat: config.WeChatConfig{AppID: "wx", Secret: "sec"}}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	m := NewMiniProgramToken()
	_, _ = m.GetAccessToken()
	m.Invalidate()
	_, _ = m.GetAccessToken()
	if calls != 2 {
		t.Fatalf("Invalidate 后应重新请求, calls=%d", calls)
	}
}

func TestMiniProgramTokenConcurrentAccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(`{"access_token":"tok-safe","expires_in":7200}`))
	}))
	t.Cleanup(srv.Close)

	oldBase := wechatTokenAPIBase
	wechatTokenAPIBase = srv.URL
	t.Cleanup(func() { wechatTokenAPIBase = oldBase })

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{WeChat: config.WeChatConfig{AppID: "wx", Secret: "sec"}}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	m := NewMiniProgramToken()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tok, err := m.GetAccessToken(); err != nil || tok != "tok-safe" {
				t.Errorf("token: %q err=%v", tok, err)
			}
		}()
	}
	wg.Wait()
}

func TestMiniProgramTokenWeChatError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":40013,"errmsg":"invalid appid"}`))
	}))
	t.Cleanup(srv.Close)

	oldBase := wechatTokenAPIBase
	wechatTokenAPIBase = srv.URL
	t.Cleanup(func() { wechatTokenAPIBase = oldBase })

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{WeChat: config.WeChatConfig{AppID: "bad", Secret: "sec"}}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	_, err := NewMiniProgramToken().GetAccessToken()
	if err == nil {
		t.Fatal("微信 errcode 非 0 应返回错误")
	}
}
