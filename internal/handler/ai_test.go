package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/testutil"

	"github.com/gin-gonic/gin"
)

func setupAIRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	testutil.EnsureAppConfig()
	r := gin.New()
	h := NewAIHandler(nil, nil)
	auth := r.Group("/api").Use(func(c *gin.Context) {
		c.Set("user_id", uint64(1))
		c.Set("family_id", uint64(1))
		c.Next()
	})
	auth.POST("/ai/recommend", h.Recommend)
	auth.GET("/ai/items/:item_id", h.GetItem)
	auth.POST("/ai/items/:item_id/import-recipe", h.ImportRecipe)
	auth.POST("/ai/items/:item_id/add-order", h.AddOrder)
	return r
}

func TestAIRecommendForbiddenWhenDisabled(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{AI: config.AIConfig{RecommendEnabled: false}}
	t.Cleanup(func() { config.AppConfig = old })

	r := setupAIRouter(t)
	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/ai/recommend"},
		{http.MethodGet, "/api/ai/items/abc"},
		{http.MethodPost, "/api/ai/items/abc/import-recipe"},
		{http.MethodPost, "/api/ai/items/abc/add-order"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s: status %d body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
		var resp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Code != 403 {
			t.Fatalf("%s %s: code=%d", tc.method, tc.path, resp.Code)
		}
	}
}
