package middleware_test
import (
	"recipe-server/internal/middleware"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
	jwtPkg "recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
)

func TestAuthRequiredMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", middleware.AuthRequired(), func(c *gin.Context) {
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
	r := gin.New()
	r.GET("/protected", middleware.AuthRequired(), func(c *gin.Context) {
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

func TestAuthRequiredValidToken(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHours: 1}}
	t.Cleanup(func() { config.AppConfig = old })

	token, err := jwtPkg.Generate("test-secret", 1, 42, "openid-1", 7)
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", middleware.AuthRequired(), func(c *gin.Context) {
		if middleware.GetUserID(c) != 42 {
			t.Fatalf("user_id: got %d", middleware.GetUserID(c))
		}
		if middleware.GetFamilyID(c) != 7 {
			t.Fatalf("family_id: got %d", middleware.GetFamilyID(c))
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

func TestOptionalAuthWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/recipes", middleware.OptionalAuth(), func(c *gin.Context) {
		if middleware.GetUserID(c) != 0 || middleware.GetFamilyID(c) != 0 {
			t.Fatalf("无 token 时应为 0, user=%d family=%d", middleware.GetUserID(c), middleware.GetFamilyID(c))
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

	token, _ := jwtPkg.Generate("test-secret", 1, 9, "oid", 3)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/recipes", middleware.OptionalAuth(), func(c *gin.Context) {
		if middleware.GetUserID(c) != 9 || middleware.GetFamilyID(c) != 3 {
			t.Fatalf("user=%d family=%d", middleware.GetUserID(c), middleware.GetFamilyID(c))
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
