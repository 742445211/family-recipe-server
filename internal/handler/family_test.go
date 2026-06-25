package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
	jwtPkg "recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
)

func TestFamilyMembersRejectsNonMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testutil.EnsureAppConfig()
	db := testutil.SetupTestDB(t)
	_, familyID := testutil.SeedUserAndFamily(t, db)

	outsider := model.User{OpenID: "outsider", Nickname: "外人"}
	db.Create(&outsider)

	token, _ := jwtPkg.Generate(config.AppConfig.JWT.Secret, 24, outsider.ID, outsider.OpenID, 0)
	h := NewFamilyHandler(db)
	r := gin.New()
	r.GET("/api/families/:id/members", middleware.AuthRequired(db), h.Members)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/families/"+jsonNumber(familyID)+"/members", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthUpdateProfileRejectsNonMemberFamily(t *testing.T) {
	r, h := setupAuthRouter(t)
	user := model.User{OpenID: "upd2-oid", Nickname: "u"}
	h.db.Create(&user)
	other := model.Family{Name: "他人家庭", InviteCode: "OTH001"}
	h.db.Create(&other)

	token, _ := jwtPkg.Generate(config.AppConfig.JWT.Secret, 24, user.ID, user.OpenID, 0)
	fid := other.ID
	body, _ := json.Marshal(map[string]any{"current_family_id": fid})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/users/me", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
}

func jsonNumber(n uint64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
