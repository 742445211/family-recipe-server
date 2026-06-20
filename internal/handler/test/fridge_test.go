package handler_test
import (
	"recipe-server/internal/handler"
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

func setupFridgeRouter(t *testing.T) (*gin.Engine, *service.FridgeService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	disp := &fridgeTestDispatcher{ok: true}
	svc := service.NewFridgeService(db, disp)
	h := handler.NewFridgeHandler(svc)
	r := gin.New()
	auth := r.Group("/api").Use(func(c *gin.Context) {
		c.Set("user_id", uint64(1))
		c.Set("family_id", uint64(1))
		c.Next()
	})
	auth.GET("/fridge/items", h.ListItems)
	auth.POST("/fridge/items", h.CreateItems)
	auth.PUT("/fridge/items/:id", h.UpdateItem)
	auth.DELETE("/fridge/items/:id", h.DeleteItem)
	auth.POST("/fridge/scans", h.CreateScan)
	auth.GET("/fridge/scans/:id", h.GetScan)
	auth.POST("/fridge/scans/:id/confirm", h.ConfirmScan)
	return r, svc
}

type fridgeTestDispatcher struct{ ok bool }

func (f *fridgeTestDispatcher) DispatchFridgeRecognize(scanID uint64, taskID, ossKey, ossURL string) bool {
	return f.ok
}

func (f *fridgeTestDispatcher) IsWorkerConnected() bool { return f.ok }

func TestFridgeForbiddenWhenDisabled(t *testing.T) {
	old := config.AppConfig
	f := false
	config.AppConfig = &config.Config{Fridge: config.FridgeConfig{Enabled: &f}}
	t.Cleanup(func() { config.AppConfig = old })

	r, _ := setupFridgeRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/fridge/items", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d", w.Code)
	}
}

func TestFridgeListAndCreate(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = old })

	r, _ := setupFridgeRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/fridge/items",
		bytes.NewBufferString(`{"name":"牛奶","amount":"1盒","expiry_date":"2026-07-01"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("create status %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/fridge/items", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list status %d", w.Code)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) != 1 {
		t.Fatalf("list=%v", resp.Data)
	}
}

func TestFridgeNoFamily(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = old })

	gin.SetMode(gin.TestMode)
	h := handler.NewFridgeHandler(service.NewFridgeService(testutil.SetupTestDB(t), nil))
	r := gin.New()
	r.GET("/api/fridge/items", func(c *gin.Context) {
		c.Set("user_id", uint64(1))
		c.Set("family_id", uint64(0))
		h.ListItems(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/fridge/items", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}

func TestFridgeConfirmScan(t *testing.T) {
	old := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = old })

	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	testutil.SeedUserAndFamily(t, db)
	svc := service.NewFridgeService(db, &fridgeTestDispatcher{ok: true})
	s, err := svc.CreateScan(1, 1, "k.jpg", "https://u")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyRecognizeResult(s.ID, []byte(`{"items":[{"name":"白菜"}]}`)); err != nil {
		t.Fatal(err)
	}

	h := handler.NewFridgeHandler(svc)
	r := gin.New()
	r.POST("/api/fridge/scans/:id/confirm", func(c *gin.Context) {
		c.Set("user_id", uint64(1))
		c.Set("family_id", uint64(1))
		h.ConfirmScan(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/fridge/scans/1/confirm",
		bytes.NewBufferString(`{"items":[{"name":"白菜","amount":"1颗"}]}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
}
