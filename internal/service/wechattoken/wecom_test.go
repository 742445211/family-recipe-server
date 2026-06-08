package wechattoken

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
)

func TestWecomTokenFetchAndCache(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("corpid") != "corp-id" {
			t.Fatalf("corpid: %q", r.URL.Query().Get("corpid"))
		}
		_, _ = w.Write([]byte(`{"access_token":"wecom-tok","expires_in":7200}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				CorpID: "corp-id", Secret: "corp-secret", APIBase: srv.URL,
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	w := NewWecomToken()
	tok1, err := w.GetAccessToken()
	if err != nil || tok1 != "wecom-tok" {
		t.Fatalf("first: %q err=%v", tok1, err)
	}
	tok2, err := w.GetAccessToken()
	if err != nil || tok2 != "wecom-tok" {
		t.Fatalf("cached: %q err=%v", tok2, err)
	}
	if calls != 1 {
		t.Fatalf("应缓存 token, calls=%d", calls)
	}
}

func TestWecomTokenTrimsAPIBaseSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"ok","expires_in":7200}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				CorpID: "c", Secret: "s", APIBase: srv.URL + "/",
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	tok, err := NewWecomToken().GetAccessToken()
	if err != nil || tok != "ok" {
		t.Fatalf("got %q err=%v", tok, err)
	}
}
