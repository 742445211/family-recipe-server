package handler_test
import (
	"recipe-server/internal/handler"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"

	"github.com/gin-gonic/gin"
)

func TestAppFeaturesAIRecommendEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.AppConfig
	config.AppConfig = &config.Config{AI: config.AIConfig{RecommendEnabled: true}}
	t.Cleanup(func() { config.AppConfig = old })

	r := gin.New()
	r.GET("/api/app/features", handler.NewAppHandler().Features)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/app/features", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			AIRecommend bool `json:"ai_recommend"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Data.AIRecommend {
		t.Fatal("ai_recommend 应为 true")
	}
}

func TestAppFeaturesAIRecommendDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.AppConfig
	config.AppConfig = &config.Config{AI: config.AIConfig{RecommendEnabled: false}}
	t.Cleanup(func() { config.AppConfig = old })

	r := gin.New()
	r.GET("/api/app/features", handler.NewAppHandler().Features)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/app/features", nil))

	var resp struct {
		Data struct {
			AIRecommend bool `json:"ai_recommend"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.AIRecommend {
		t.Fatal("ai_recommend 应为 false")
	}
}
