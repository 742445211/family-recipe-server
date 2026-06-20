package handler_test
import (
	"recipe-server/internal/handler"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/service"
	"recipe-server/internal/testutil"
	jwtPkg "recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
)

func setupAuthRouter(t *testing.T) (*gin.Engine, *handler.AuthHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	testutil.EnsureAppConfig()
	db := testutil.SetupTestDB(t)
	h := handler.NewAuthHandler(db)
	r := gin.New()
	r.POST("/api/auth/login", h.Login)
	auth := r.Group("/api").Use(middleware.AuthRequired())
	auth.GET("/users/me", h.GetProfile)
	auth.PUT("/users/me", h.UpdateProfile)
	return r, h, db
}

func TestAuthLoginCreatesUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"openid":"login-oid","session_key":"sk","errcode":0}`))
	}))
	t.Cleanup(srv.Close)

	oldBase := service.WechatAPIBaseForTest()
	service.SetWechatAPIBaseForTest(srv.URL)
	t.Cleanup(func() { service.SetWechatAPIBaseForTest(oldBase) })

	r, _, _ := setupAuthRouter(t)
	body, _ := json.Marshal(map[string]string{"code": "wx-code", "nickname": "新用户"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body)))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Token    string `json:"token"`
			Nickname string `json:"nickname"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 || resp.Data.Token == "" || resp.Data.Nickname != "新用户" {
		t.Fatalf("resp: %+v", resp)
	}
}

func TestAuthLoginMissingCode(t *testing.T) {
	r, _, _ := setupAuthRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte(`{}`))))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestAuthGetProfile(t *testing.T) {
	r, _, db := setupAuthRouter(t)

	user := model.User{OpenID: "profile-oid", Nickname: "厨师"}
	db.Create(&user)
	family := model.Family{Name: "家", InviteCode: "PRF001"}
	db.Create(&family)
	db.Create(&model.FamilyMember{FamilyID: family.ID, UserID: user.ID, Role: "owner", IsChef: true})
	fid := family.ID
	db.Model(&user).Update("current_family_id", fid)

	token, _ := jwtPkg.Generate(config.AppConfig.JWT.Secret, 24, user.ID, user.OpenID, family.ID)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			IsChef bool `json:"is_chef"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Data.IsChef {
		t.Fatal("应识别厨师身份")
	}
}

func TestAuthUpdateProfile(t *testing.T) {
	r, _, db := setupAuthRouter(t)
	user := model.User{OpenID: "upd-oid", Nickname: "旧名"}
	db.Create(&user)

	token, _ := jwtPkg.Generate(config.AppConfig.JWT.Secret, 24, user.ID, user.OpenID, 0)
	body, _ := json.Marshal(map[string]string{"nickname": "新名"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/users/me", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var updated model.User
	db.First(&updated, user.ID)
	if updated.Nickname != "新名" {
		t.Fatalf("nickname: %q", updated.Nickname)
	}
}
