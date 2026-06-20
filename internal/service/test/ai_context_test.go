package service_test
import (
	"recipe-server/internal/service"
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
	if service.FormatHistorySummary(nil) != "暂无历史记录" {
		t.Fatal("空历史应返回默认文案")
	}
	got := service.FormatHistorySummary([]string{"红烧肉", "清炒时蔬"})
	if !strings.Contains(got, "红烧肉") || !strings.Contains(got, "清炒时蔬") {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestFormatContextBlock(t *testing.T) {
	if service.FormatContextBlock(nil) != "" {
		t.Fatal("nil ctx 应返回空")
	}
	block := service.FormatContextBlock(&service.AIRecommendContext{
		RecipeNames:  []string{"番茄炒蛋"},
		OrderHistory: []string{"红烧肉"},
		WeatherLine:  "成都 28°C 晴",
		Meal:         service.MealSlot{Type: "dinner", Name: "晚餐"},
	})
	for _, want := range []string{"番茄炒蛋", "红烧肉", "成都 28°C 晴", "禁止推荐", "晚餐"} {
		if !strings.Contains(block, want) {
			t.Fatalf("block 应包含 %q: %s", want, block)
		}
	}
	if !strings.Contains(service.FormatContextBlock(&service.AIRecommendContext{}), "可自由推荐新菜") {
		t.Fatal("无已有菜谱时应提示可自由推荐")
	}
}

func TestInferMealSlot(t *testing.T) {
	cases := []struct {
		hour     int
		wantType string
		wantName string
	}{
		{7, "breakfast", "早餐"},
		{12, "lunch", "午餐"},
		{18, "dinner", "晚餐"},
		{23, "supper", "宵夜"},
		{2, "supper", "宵夜"},
	}
	for _, c := range cases {
		ms := service.InferMealSlot(time.Date(2026, 6, 8, c.hour, 0, 0, 0, time.Local))
		if ms.Type != c.wantType || ms.Name != c.wantName {
			t.Fatalf("hour=%d got %+v want %s/%s", c.hour, ms, c.wantType, c.wantName)
		}
	}
}

func TestNormalizeMealSlot(t *testing.T) {
	cases := map[string]string{
		"breakfast": "breakfast",
		"早餐":        "breakfast",
		"lunch":     "lunch",
		"午餐":        "lunch",
		"dinner":    "dinner",
		"晚餐":        "dinner",
		"supper":    "supper",
		"宵夜":        "supper",
		"夜宵":        "supper",
	}
	for in, want := range cases {
		ms, ok := service.NormalizeMealSlot(in)
		if !ok || ms.Type != want {
			t.Fatalf("service.NormalizeMealSlot(%q)=%+v ok=%v want %s", in, ms, ok, want)
		}
	}
	if _, ok := service.NormalizeMealSlot("乱填"); ok {
		t.Fatal("无法识别的餐次应返回 ok=false")
	}
	if _, ok := service.NormalizeMealSlot(""); ok {
		t.Fatal("空餐次应返回 ok=false")
	}
}

func TestAIContextServiceBuildSetsMealSlot(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_, familyID := testutil.SeedUserAndFamily(t, db)
	svc := service.NewAIContextService(db, nil)
	ctx, err := svc.Build(familyID)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Meal.Type == "" || ctx.Meal.Name == "" {
		t.Fatalf("Build 应根据当前时间推断餐次: %+v", ctx.Meal)
	}
}

func TestAIContextServiceBuild(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	db.Create(&model.Recipe{Name: "  番茄炒蛋  ", FamilyID: familyID, CreatorID: userID, Ingredients: `[]`, Steps: `[]`})
	db.Create(&model.Recipe{Name: "番茄炒蛋", FamilyID: familyID, CreatorID: userID, Ingredients: `[]`, Steps: `[]`})

	os := service.NewOrderService(db)
	r1 := &model.Recipe{Name: "  番茄炒蛋  ", FamilyID: familyID, CreatorID: userID, Ingredients: `[]`, Steps: `[]`}
	db.Create(r1)
	_, _ = os.Add(familyID, r1.ID, "dinner", userID, time.Now().Format("2006-01-02"), "", 1)

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
	weather := service.NewWeatherService(store, &http.Client{Timeout: 5 * time.Second})
	service.SetWeatherAPIBaseForTest(weather, srv.URL)
	svc := service.NewAIContextService(db, weather)
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
	svc := service.NewAIContextService(db, nil)
	ctx, err := svc.Build(familyID)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.WeatherLine != "天气信息暂不可用" {
		t.Fatalf("无 weather 服务时应使用默认文案: %q", ctx.WeatherLine)
	}
}
