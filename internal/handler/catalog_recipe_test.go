package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/service"
	"recipe-server/internal/testutil"

	"github.com/gin-gonic/gin"
)

func setupCatalogRouter(t *testing.T) (*gin.Engine, *service.CatalogRecipeService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	svc := service.NewCatalogRecipeService(db, nil, nil)
	h := NewCatalogRecipeHandler(svc)
	r := gin.New()
	auth := r.Group("/api").Use(func(c *gin.Context) {
		c.Set("user_id", uint64(1))
		c.Set("family_id", uint64(1))
		c.Next()
	})
	auth.POST("/catalog-recipes/lookup", h.Lookup)
	auth.GET("/catalog-recipes/:id", h.Get)
	auth.POST("/catalog-recipes/:id/use", h.Use)
	return r, svc
}

func TestCatalogLookupForbiddenWhenDisabled(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{AI: config.AIConfig{RecommendEnabled: false}}
	t.Cleanup(func() { config.AppConfig = old })

	r, _ := setupCatalogRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/catalog-recipes/lookup",
		bytes.NewBufferString(`{"name":"番茄炒蛋"}`)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
}

func TestCatalogLookupBadRequest(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{AI: config.AIConfig{RecommendEnabled: true}}
	t.Cleanup(func() { config.AppConfig = old })

	r, _ := setupCatalogRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/catalog-recipes/lookup",
		bytes.NewBufferString(`{}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}

func TestCatalogLookupHitExisting(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{AI: config.AIConfig{RecommendEnabled: true}}
	t.Cleanup(func() { config.AppConfig = old })

	r, svc := setupCatalogRouter(t)
	_, err := svc.SaveFromAI(service.AIRecommendItemInput{
		Name: "缓存菜", Category: "荤菜", Ingredients: "[]", Seasonings: "[]", Steps: `["煮"]`,
	}, service.CatalogSourceAISearch, "经典做法")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/catalog-recipes/lookup",
		bytes.NewBufferString(`{"name":"缓存菜"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Generated bool `json:"generated"`
			Variants  []struct {
				ID   uint64 `json:"id"`
				Name string `json:"name"`
			} `json:"variants"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Generated || len(resp.Data.Variants) != 1 {
		t.Fatalf("%+v", resp.Data)
	}
}

func TestAppFeaturesCatalogRecipe(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{AI: config.AIConfig{RecommendEnabled: true}}
	t.Cleanup(func() { config.AppConfig = old })

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.GET("/api/app/features", NewAppHandler().Features)
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/app/features", nil))

	var resp struct {
		Data struct {
			CatalogRecipe bool `json:"catalog_recipe"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Data.CatalogRecipe {
		t.Fatal("catalog_recipe should be true")
	}
}
