package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/cache"
	"recipe-server/internal/model"

	"github.com/alicebob/miniredis/v2"
)

func TestParseAIRecommendJSON(t *testing.T) {
	raw := `{"items":[{"name":"番茄炒蛋","category":"家常菜","difficulty":"easy","cook_time":15,"ingredients":"[{\"name\":\"番茄\",\"amount\":\"2个\"}]","seasonings":"[]","steps":"[\"切块\"]","tips":"先炒蛋","reason":"快手"}]}`
	items, err := parseAIRecommendJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "番茄炒蛋" {
		t.Fatalf("%+v", items)
	}
}

func TestParseAIRecommendJSONWithJSONArrayFields(t *testing.T) {
	raw := `{"items":[{"name":"番茄炒蛋","category":"家常菜","difficulty":"easy","cook_time":15,"ingredients":[{"name":"番茄","amount":"2个"},{"name":"鸡蛋","amount":"3个"}],"seasonings":[],"steps":["打蛋","番茄切块","合炒"],"tips":"少油","reason":"快手"}]}`
	items, err := parseAIRecommendJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%d", len(items))
	}
	if !strings.Contains(items[0].Ingredients, "番茄") {
		t.Fatalf("ingredients=%s", items[0].Ingredients)
	}
	if !strings.Contains(items[0].Steps, "打蛋") {
		t.Fatalf("steps=%s", items[0].Steps)
	}
}

func TestParseAIRecommendJSONStepsWithColon(t *testing.T) {
	raw := `{"items":[{"name":"红烧排骨","category":"荤菜","difficulty":"medium","cook_time":40,"ingredients":[{"name":"排骨","amount":"500g"}],"seasonings":[],"steps":["焯水: 去血沫","炒糖色: 小火","炖煮: 40分钟"],"tips":"","reason":"暖胃"}]}`
	items, err := parseAIRecommendJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Name != "红烧排骨" {
		t.Fatalf("%+v", items[0])
	}
}

func TestParseAIRecommendJSONStripsMarkdown(t *testing.T) {
	raw := "```json\n{\"items\":[{\"name\":\"A\",\"category\":\"\",\"difficulty\":\"easy\",\"cook_time\":10,\"ingredients\":\"[]\",\"seasonings\":\"[]\",\"steps\":\"[]\",\"tips\":\"\",\"reason\":\"\"}]}\n```"
	items, err := parseAIRecommendJSON(raw)
	if err != nil || items[0].Name != "A" {
		t.Fatalf("%v %+v", err, items)
	}
}

func TestAIRecommendServiceRecommend(t *testing.T) {
	db := setupTestDB(t)
	uid, fid := seedUserAndFamily(t, db)
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)

	llmBody := `{"items":[{"name":"新菜","category":"荤菜","difficulty":"medium","cook_time":30,"ingredients":"[{\"name\":\"肉\",\"amount\":\"500g\"}]","seasonings":"[]","steps":"[\"煮\"]","tips":"","reason":"适合今天"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": llmBody}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	config.AppConfig = &config.Config{
		AI: config.AIConfig{
			APIKey:                 "k",
			BaseURL:                srv.URL,
			Model:                  "test",
			RecommendCacheTTLHours: 24,
			RecommendCount:         5,
			RateLimit:              config.AIRateLimitConfig{Enabled: false},
		},
		Weather: config.WeatherConfig{Enabled: false},
	}

	store := cache.NewRedisCache(mr.Addr(), "", 0)
	weather := NewWeatherService(store, nil)
	ctxSvc := NewAIContextService(db, weather)
	ai := NewAIServiceWithClient(&http.Client{})
	ai.baseURL = srv.URL

	svc := NewAIRecommendService(db, store, ai, ctxSvc, NewAIRateLimitService(store))
	result, err := svc.Recommend(context.Background(), fid, uid)
	if err != nil {
		t.Fatal(err)
	}
	if result.BatchID == "" || len(result.Items) != 1 {
		t.Fatalf("%+v", result)
	}
	if result.Items[0].ItemID == "" || result.Items[0].Name != "新菜" {
		t.Fatalf("%+v", result.Items[0])
	}

	var draft AIRecipeDraft
	if err := store.GetJSON(context.Background(), aiItemKey(result.Items[0].ItemID), &draft); err != nil {
		t.Fatal(err)
	}
	if draft.FamilyID != fid || draft.Name != "新菜" {
		t.Fatalf("%+v", draft)
	}
}

func TestAIRecommendMatchExistingRecipe(t *testing.T) {
	db := setupTestDB(t)
	uid, fid := seedUserAndFamily(t, db)
	existing := &model.Recipe{Name: "红烧肉", FamilyID: fid, CreatorID: uid, Ingredients: `[]`, Steps: `[]`}
	db.Create(existing)

	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	body := `{"items":[{"name":"红烧肉","category":"","difficulty":"easy","cook_time":20,"ingredients":"[]","seasonings":"[]","steps":"[]","tips":"","reason":"常点菜"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + strconvQuote(body) + `}}]}`))
	}))
	t.Cleanup(srv.Close)

	config.AppConfig = &config.Config{
		AI: config.AIConfig{BaseURL: srv.URL, Model: "t", RateLimit: config.AIRateLimitConfig{Enabled: false}},
		Weather: config.WeatherConfig{Enabled: false},
	}
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	ai := NewAIServiceWithClient(&http.Client{})
	ai.baseURL = srv.URL
	svc := NewAIRecommendService(db, store, ai, NewAIContextService(db, NewWeatherService(store, nil)), NewAIRateLimitService(store))
	result, err := svc.Recommend(context.Background(), fid, uid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Items[0].ExistingRecipeID == nil || *result.Items[0].ExistingRecipeID != existing.ID {
		t.Fatalf("%+v", result.Items[0])
	}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestImportAIRecipeFromCache(t *testing.T) {
	db := setupTestDB(t)
	uid, fid := seedUserAndFamily(t, db)
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	itemID := "test-item-1"
	draft := AIRecipeDraft{
		ItemID: itemID, FamilyID: fid, Name: "AI菜", Category: "家常菜",
		Difficulty: "easy", CookTime: 10,
		Ingredients: `[{"name":"蛋","amount":"2"}]`, Seasonings: `[]`, Steps: `["炒"]`, Tips: "快",
	}
	_ = store.SetJSON(context.Background(), aiItemKey(itemID), draft, 0)

	svc := NewAIRecommendService(db, store, nil, nil, nil)
	rec, err := svc.ImportRecipe(context.Background(), itemID, fid, uid)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Name != "AI菜" || rec.FamilyID != fid {
		t.Fatalf("%+v", rec)
	}
}

func TestAddOrderFromAIItem(t *testing.T) {
	db := setupTestDB(t)
	uid, fid := seedUserAndFamily(t, db)
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	itemID := "order-item-1"
	draft := AIRecipeDraft{
		ItemID: itemID, FamilyID: fid, Name: "点菜AI菜", Category: "荤菜",
		Difficulty: "medium", CookTime: 20,
		Ingredients: `[]`, Seasonings: `[]`, Steps: `[]`,
	}
	_ = store.SetJSON(context.Background(), aiItemKey(itemID), draft, 0)

	svc := NewAIRecommendService(db, store, nil, nil, nil)
	order, err := svc.AddOrderFromItem(context.Background(), itemID, fid, uid, AddOrderFromAIRequest{
		MealType: "dinner", Date: "2026-06-07", Note: "少盐",
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.Recipe == nil || order.Recipe.Name != "点菜AI菜" {
		t.Fatalf("%+v", order)
	}
}

func TestGetAIItemWrongFamily(t *testing.T) {
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	_ = store.SetJSON(context.Background(), aiItemKey("x"), AIRecipeDraft{FamilyID: 1, Name: "A"}, 0)
	svc := NewAIRecommendService(nil, store, nil, nil, nil)
	_, err := svc.GetItem(context.Background(), "x", 2)
	if err == nil || !strings.Contains(err.Error(), "无权") {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
