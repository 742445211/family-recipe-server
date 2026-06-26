package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/middleware"
	"recipe-server/internal/service"
	"recipe-server/internal/service/wechattoken"
	"recipe-server/internal/testutil"
	jwtPkg "recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
)

func TestRecipeCreateBlocksUnsafeContent(t *testing.T) {
	secSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errcode": 0,
			"result":  map[string]any{"suggest": "risky"},
		})
	}))
	t.Cleanup(secSrv.Close)

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
	}))
	t.Cleanup(tokenSrv.Close)

	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		JWT:    config.JWTConfig{Secret: "test-secret", ExpireHours: 1},
		WeChat: config.WeChatConfig{AppID: "wx", Secret: "sec", SecCheckEnabled: boolPtr(true)},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	oldSecBase := service.SecCheckAPIBaseForTest()
	service.SetSecCheckAPIBaseForTest(secSrv.URL)
	t.Cleanup(func() { service.SetSecCheckAPIBaseForTest(oldSecBase) })

	oldTokenBase := wechattoken.WechatTokenAPIBaseForTest()
	wechattoken.SetWechatTokenAPIBaseForTest(tokenSrv.URL)
	t.Cleanup(func() { wechattoken.SetWechatTokenAPIBaseForTest(oldTokenBase) })

	service.DefaultSecCheck = service.NewSecCheckService(wechattoken.NewMiniProgramToken())

	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	token, _ := jwtPkg.Generate("test-secret", 1, userID, "openid-test", familyID)

	gin.SetMode(gin.TestMode)
	h := NewRecipeHandler(db)
	r := gin.New()
	r.POST("/api/recipes", middleware.AuthRequired(db), h.Create)

	body, _ := json.Marshal(map[string]any{
		"name":        "测试菜",
		"category":    "家常菜",
		"ingredients": `[{"name":"违规词","amount":"1"}]`,
		"steps":       `["步骤1"]`,
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/recipes", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != service.ErrContentUnsafe.Error() {
		t.Fatalf("msg=%v", resp["msg"])
	}
}

func boolPtr(v bool) *bool { return &v }
