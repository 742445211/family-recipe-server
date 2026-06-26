package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/model"
	"recipe-server/internal/service/wechattoken"
)

func TestExtractStringsFromJSON(t *testing.T) {
	t.Parallel()
	got := extractStringsFromJSON(
		`[{"name":"番茄","amount":"2个"}]`,
		`["切菜","下锅"]`,
	)
	if len(got) != 4 {
		t.Fatalf("want 4 strings, got %v", got)
	}
}

func TestSecCheckServiceBlocksRiskyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wxa/msg_sec_check" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errcode": 0,
			"errmsg":  "ok",
			"result":  map[string]any{"suggest": "risky", "label": 20001},
		})
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		WeChat: config.WeChatConfig{AppID: "wx-test", Secret: "sec", SecCheckEnabled: boolPtr(true)},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	oldBase := secCheckAPIBase
	secCheckAPIBase = srv.URL
	t.Cleanup(func() { secCheckAPIBase = oldBase })

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
	}))
	t.Cleanup(tokenSrv.Close)

	oldTokenBase := wechattoken.WechatTokenAPIBaseForTest()
	wechattoken.SetWechatTokenAPIBaseForTest(tokenSrv.URL)
	t.Cleanup(func() { wechattoken.SetWechatTokenAPIBaseForTest(oldTokenBase) })

	svc := NewSecCheckService(wechattoken.NewMiniProgramToken())
	err := svc.CheckTexts("openid-1", SecCheckSceneProfile, "违规测试")
	if err == nil {
		t.Fatal("expected unsafe content error")
	}
	if err != ErrContentUnsafe {
		t.Fatalf("want ErrContentUnsafe, got %v", err)
	}
}

func TestSecCheckServicePassesSafeContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errcode": 0,
			"errmsg":  "ok",
			"result":  map[string]any{"suggest": "pass", "label": 100},
		})
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		WeChat: config.WeChatConfig{AppID: "wx-test", Secret: "sec", SecCheckEnabled: boolPtr(true)},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	oldBase := secCheckAPIBase
	secCheckAPIBase = srv.URL
	t.Cleanup(func() { secCheckAPIBase = oldBase })

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
	}))
	t.Cleanup(tokenSrv.Close)

	oldTokenBase := wechattoken.WechatTokenAPIBaseForTest()
	wechattoken.SetWechatTokenAPIBaseForTest(tokenSrv.URL)
	t.Cleanup(func() { wechattoken.SetWechatTokenAPIBaseForTest(oldTokenBase) })

	svc := NewSecCheckService(wechattoken.NewMiniProgramToken())
	if err := svc.CheckTexts("openid-1", SecCheckSceneProfile, "红烧肉"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCheckRecipeUGCSkipsWhenDisabled(t *testing.T) {
	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{WeChat: config.WeChatConfig{SecCheckEnabled: boolPtr(false)}}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	svc := NewSecCheckService(nil)
	if err := svc.CheckRecipeUGC("", &model.Recipe{Name: "anything"}); err != nil {
		t.Fatalf("disabled should skip: %v", err)
	}
}

func TestSecCheckImageBlocksRiskyImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wxa/img_sec_check" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Fatalf("content-type: %s", r.Header.Get("Content-Type"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 87014, "errmsg": "risky content"})
	}))
	t.Cleanup(srv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		WeChat: config.WeChatConfig{AppID: "wx-test", Secret: "sec", SecCheckEnabled: boolPtr(true)},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	oldBase := secCheckAPIBase
	secCheckAPIBase = srv.URL
	t.Cleanup(func() { secCheckAPIBase = oldBase })

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
	}))
	t.Cleanup(tokenSrv.Close)

	oldTokenBase := wechattoken.WechatTokenAPIBaseForTest()
	wechattoken.SetWechatTokenAPIBaseForTest(tokenSrv.URL)
	t.Cleanup(func() { wechattoken.SetWechatTokenAPIBaseForTest(oldTokenBase) })

	svc := NewSecCheckService(wechattoken.NewMiniProgramToken())
	err := svc.CheckImage("openid-1", []byte("fakejpeg"), "cover.jpg")
	if !errors.Is(err, ErrContentUnsafe) {
		t.Fatalf("want ErrContentUnsafe, got %v", err)
	}
}

func TestSecCheckImageRejectsOversize(t *testing.T) {
	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		WeChat: config.WeChatConfig{AppID: "wx-test", Secret: "sec", SecCheckEnabled: boolPtr(true)},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	svc := NewSecCheckService(nil)
	data := make([]byte, maxImgSecCheckBytes+1)
	err := svc.CheckImage("openid-1", data, "big.jpg")
	if !errors.Is(err, ErrImageTooLargeForSecCheck) {
		t.Fatalf("want ErrImageTooLargeForSecCheck, got %v", err)
	}
}
