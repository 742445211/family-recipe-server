package wechattoken

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestWecomGetUseridByMobile(t *testing.T) {
	var gotMobile string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/gettoken"):
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		case strings.Contains(r.URL.Path, "/user/getuserid"):
			if r.URL.Query().Get("access_token") != "tok" {
				t.Fatalf("缺少 access_token: %s", r.URL.RawQuery)
			}
			b, _ := io.ReadAll(r.Body)
			gotMobile = string(b)
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","userid":"zhangsan"}`))
		default:
			t.Fatalf("未预期的请求: %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{
				CorpID: "c", Secret: "s", APIBase: srv.URL,
			},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	userid, err := NewWecomToken().GetUseridByMobile("13800138000")
	if err != nil {
		t.Fatalf("GetUseridByMobile: %v", err)
	}
	if userid != "zhangsan" {
		t.Fatalf("userid: got %q", userid)
	}
	if !strings.Contains(gotMobile, "13800138000") {
		t.Fatalf("请求体应包含手机号: %s", gotMobile)
	}
}

func TestWecomGetUseridByMobileNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/gettoken") {
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
			return
		}
		_, _ = w.Write([]byte(`{"errcode":60111,"errmsg":"userid not found"}`))
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WecomWorkbench: config.NotificationWecom{CorpID: "c", Secret: "s", APIBase: srv.URL},
		},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	if _, err := NewWecomToken().GetUseridByMobile("13800138000"); err == nil {
		t.Fatal("查询失败应返回错误")
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
