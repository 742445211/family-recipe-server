package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/testutil"
	jwtPkg "recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
)

func TestAuthRequiredMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	r := gin.New()
	r.GET("/protected", AuthRequired(db), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d", w.Code)
	}
}

func TestAuthRequiredInvalidBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	r := gin.New()
	r.GET("/protected", AuthRequired(db), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Token abc")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d", w.Code)
	}
}

func TestAuthRequiredValidTokenWithMembership(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHours: 1}}
	t.Cleanup(func() { config.AppConfig = old })

	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	token, err := jwtPkg.Generate("test-secret", 1, userID, "openid-1", familyID)
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", AuthRequired(db), func(c *gin.Context) {
		if GetUserID(c) != userID {
			t.Fatalf("user_id: got %d", GetUserID(c))
		}
		if GetFamilyID(c) != familyID {
			t.Fatalf("family_id: got %d", GetFamilyID(c))
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthRequiredFallsBackWhenInvalidFamilyClaim(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHours: 1}}
	t.Cleanup(func() { config.AppConfig = old })

	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	token, err := jwtPkg.Generate("test-secret", 1, userID, "openid-1", 99999)
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", AuthRequired(db), func(c *gin.Context) {
		if GetFamilyID(c) != familyID {
			t.Fatalf("invalid claim should fall back to current_family_id %d, got %d", familyID, GetFamilyID(c))
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestOptionalAuthWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	r := gin.New()
	r.GET("/recipes", OptionalAuth(db), func(c *gin.Context) {
		if GetUserID(c) != 0 || GetFamilyID(c) != 0 {
			t.Fatalf("无 token 时应为 0, user=%d family=%d", GetUserID(c), GetFamilyID(c))
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/recipes", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestOptionalAuthWithValidToken(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHours: 1}}
	t.Cleanup(func() { config.AppConfig = old })

	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	token, _ := jwtPkg.Generate("test-secret", 1, userID, "oid", familyID)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/recipes", OptionalAuth(db), func(c *gin.Context) {
		if GetUserID(c) != userID || GetFamilyID(c) != familyID {
			t.Fatalf("user=%d family=%d", GetUserID(c), GetFamilyID(c))
		}
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/recipes", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}
