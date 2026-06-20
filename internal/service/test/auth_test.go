package service_test
import (
	"recipe-server/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
)

func TestCode2SessionSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("js_code") != "valid-code" {
			t.Fatalf("js_code: %q", r.URL.Query().Get("js_code"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openid":"oid-123","session_key":"sk","unionid":"uid-1","errcode":0}`))
	}))
	t.Cleanup(srv.Close)

	oldBase := service.WechatAPIBaseForTest()
	service.SetWechatAPIBaseForTest(srv.URL)
	t.Cleanup(func() { service.SetWechatAPIBaseForTest(oldBase) })

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{WeChat: config.WeChatConfig{AppID: "wx-test", Secret: "secret"}}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	session, err := service.Code2Session("valid-code")
	if err != nil {
		t.Fatal(err)
	}
	if session.OpenID != "oid-123" || session.UnionID != "uid-1" {
		t.Fatalf("session: %+v", session)
	}
}

func TestCode2SessionWeChatError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	}))
	t.Cleanup(srv.Close)

	oldBase := service.WechatAPIBaseForTest()
	service.SetWechatAPIBaseForTest(srv.URL)
	t.Cleanup(func() { service.SetWechatAPIBaseForTest(oldBase) })

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{WeChat: config.WeChatConfig{AppID: "wx", Secret: "sec"}}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	_, err := service.Code2Session("bad-code")
	if err == nil {
		t.Fatal("微信 errcode 非 0 应返回错误")
	}
}
