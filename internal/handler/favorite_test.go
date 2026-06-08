package handler

import (
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

func TestFavoriteAddListRemove(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	recipe := model.Recipe{Name: "收藏菜", CreatorID: userID, FamilyID: familyID}
	db.Create(&recipe)

	gin.SetMode(gin.TestMode)
	h := NewFavoriteHandler(db)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	r.POST("/api/favorites/:id", h.Add)
	r.DELETE("/api/favorites/:id", h.Remove)
	r.GET("/api/favorites", h.List)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/favorites/1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("add status %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/favorites", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list status %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/favorites/1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("idempotent add status %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/favorites/1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("remove status %d", w.Code)
	}

	var count int64
	db.Model(&model.Favorite{}).Where("user_id = ?", userID).Count(&count)
	if count != 0 {
		t.Fatalf("软删除后 active count: %d", count)
	}
}

func TestFavoriteRequiresAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewFavoriteHandler(testutil.SetupTestDB(t))
	r := gin.New()
	r.Use(middleware.AuthRequired())
	r.GET("/api/favorites", h.List)

	token, _ := jwtPkg.Generate("test-secret", 1, 1, "oid", 0)
	old := config.AppConfig
	config.AppConfig = &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHours: 1}}
	t.Cleanup(func() { config.AppConfig = old })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/favorites", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}
