package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/cache"
	"recipe-server/internal/model"
	"recipe-server/internal/service"
	"recipe-server/internal/testutil"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func setupBlindBoxHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB, uint64, uint64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	testutil.EnsureAppConfig()
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	mr, _ := miniredis.Run()
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	t.Cleanup(mr.Close)
	enabled := true
	config.AppConfig.BlindBox.Enabled = &enabled
	config.AppConfig.BlindBox.RateLimit = config.BlindBoxRateLimitConfig{Enabled: false}
	blindBox := service.NewBlindBoxService(db, store)
	h := NewOrderHandler(db, nil, blindBox)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("family_id", familyID)
		c.Next()
	})
	r.POST("/orders/blind-box/draw", h.DrawBlindBox)
	return r, db, userID, familyID
}

func TestDrawBlindBoxHandler(t *testing.T) {
	r, db, userID, familyID := setupBlindBoxHandlerTest(t)
	recipe := model.Recipe{Name: "盲盒菜", CreatorID: userID, FamilyID: familyID}
	if err := db.Create(&recipe).Error; err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"date":        "2026-06-12",
		"meal_type":   "dinner",
		"exclude_ids": []uint64{},
	})
	req := httptest.NewRequest(http.MethodPost, "/orders/blind-box/draw", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestDrawBlindBoxDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	disabled := false
	config.AppConfig = &config.Config{BlindBox: config.BlindBoxConfig{Enabled: &disabled}}
	h := NewOrderHandler(nil, nil, service.NewBlindBoxService(nil, nil))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	h.DrawBlindBox(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d", w.Code)
	}
}
