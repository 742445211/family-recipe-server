package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"recipe-server/config"
	"recipe-server/internal/cache"
	"recipe-server/internal/model"
	"recipe-server/internal/testutil"

	"github.com/alicebob/miniredis/v2"
)

func TestFormatHistorySummary(t *testing.T) {
	if FormatHistorySummary(nil) != "暂无历史记录" {
		t.Fatal("空历史应返回默认文案")
	}
	got := FormatHistorySummary([]string{"红烧肉", "清炒时蔬"})
	if !strings.Contains(got, "红烧肉") || !strings.Contains(got, "清炒时蔬") {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestFormatContextBlock(t *testing.T) {
	if FormatContextBlock(nil) != "" {
		t.Fatal("nil ctx 应返回空")
	}
	block := FormatContextBlock(&AIRecommendContext{
		RecipeNames:  []string{"番茄炒蛋"},
		OrderHistory: []string{"红烧肉"},
		WeatherLine:  "成都 28°C 晴",
	})
	for _, want := range []string{"番茄炒蛋", "红烧肉", "成都 28°C 晴", "禁止推荐"} {
		if !strings.Contains(block, want) {
			t.Fatalf("block 应包含 %q: %s", want, block)
		}
	}
	if !strings.Contains(FormatContextBlock(&AIRecommendContext{}), "可自由推荐新菜") {
		t.Fatal("无已有菜谱时应提示可自由推荐")
	}
}

func TestAIContextServiceBuild(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	db.Create(&model.Recipe{Name: "  番茄炒蛋  ", FamilyID: familyID, CreatorID: userID, Ingredients: `[]`, Steps: `[]`})
	db.Create(&model.Recipe{Name: "番茄炒蛋", FamilyID: familyID, CreatorID: userID, Ingredients: `[]`, Steps: `[]`})

	os := NewOrderService(db)
	r1 := &model.Recipe{Name: "  番茄炒蛋  ", FamilyID: familyID, CreatorID: userID, Ingredients: `[]`, Steps: `[]`}
	db.Create(r1)
	_, _ = os.Add(familyID, r1.ID, "dinner", userID, "2026-06-08", "", 1)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"current":{"temperature_2m":25,"relative_humidity_2m":60,"weather_code":0}}`))
	}))
	t.Cleanup(srv.Close)

	config.AppConfig = &config.Config{
		Weather: config.WeatherConfig{
			Enabled: true, DefaultCity: "成都", DefaultLat: 30.57, DefaultLon: 104.06, CacheTTLHours: 3,
		},
	}
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	weather := NewWeatherService(store, &http.Client{Timeout: 5 * time.Second})
	weather.apiBase = srv.URL

	svc := NewAIContextService(db, weather)
	ctx, err := svc.Build(familyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.RecipeNames) != 1 || ctx.RecipeNames[0] != "番茄炒蛋" {
		t.Fatalf("去重菜名: %v", ctx.RecipeNames)
	}
	if len(ctx.OrderHistory) != 1 || strings.TrimSpace(ctx.OrderHistory[0]) != "番茄炒蛋" {
		t.Fatalf("历史点菜: %v", ctx.OrderHistory)
	}
	if !strings.Contains(ctx.WeatherLine, "成都") {
		t.Fatalf("weather 摘要应含城市: %q", ctx.WeatherLine)
	}
}

func TestAIContextServiceBuildWithoutWeather(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_, familyID := testutil.SeedUserAndFamily(t, db)
	svc := NewAIContextService(db, nil)
	ctx, err := svc.Build(familyID)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.WeatherLine != "天气信息暂不可用" {
		t.Fatalf("无 weather 服务时应使用默认文案: %q", ctx.WeatherLine)
	}
}
