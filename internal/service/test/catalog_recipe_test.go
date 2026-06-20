package service_test
import (
	"recipe-server/internal/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/cache"
	"recipe-server/internal/model"
	"recipe-server/internal/testutil"

	"github.com/alicebob/miniredis/v2"
)

func TestNormalizeNameKey(t *testing.T) {
	if got := service.NormalizeNameKey("  番茄炒蛋  "); got != "番茄炒蛋" {
		t.Fatalf("got %q", got)
	}
	if got := service.NormalizeNameKey("Tomato Egg"); got != "tomatoegg" {
		t.Fatalf("got %q", got)
	}
}

func TestCatalogContentHashDedup(t *testing.T) {
	h1 := service.CatalogContentHash(`[{"name":"a"}]`, "[]", `["step"]`)
	h2 := service.CatalogContentHash(`[{"name":"a"}]`, "[]", `["step"]`)
	if h1 != h2 {
		t.Fatal("same content should same hash")
	}
}

func TestCatalogSaveFromAIAndLookup(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := service.NewCatalogRecipeService(db, nil, nil)
	in := service.AIRecommendItemInput{
		Name: "番茄炒蛋", Category: "家常菜", Difficulty: "easy", CookTime: 15,
		Ingredients: `[{"name":"番茄","amount":"2"}]`, Seasonings: "[]", Steps: `["炒"]`, Tips: "快",
	}
	rec, err := svc.SaveFromAI(in, service.CatalogSourceAISearch, "经典做法")
	if err != nil || rec.ID == 0 {
		t.Fatalf("%v %+v", err, rec)
	}
	if !rec.IsDefault {
		t.Fatal("first variant should be default")
	}

	list, err := svc.LookupByNameKey(service.NormalizeNameKey("番茄炒蛋"))
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%+v err=%v", list, err)
	}

	dup, err := svc.SaveFromAI(in, service.CatalogSourceAISearch, "重复")
	if err != nil || dup.ID != rec.ID {
		t.Fatalf("hash dedup: got id=%d want %d err=%v", dup.ID, rec.ID, err)
	}
}

func TestCatalogLookupOrGenerateHitNoAI(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	config.AppConfig = &config.Config{
		AI: config.AIConfig{
			RateLimit: config.AIRateLimitConfig{
				Catalog: config.AIRateLimitScopeConfig{Enabled: true, MaxRequests: 5, WindowHours: 2},
			},
		},
	}
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	rl := service.NewAIRateLimitService(store)
	svc := service.NewCatalogRecipeService(db, nil, rl)

	in := service.AIRecommendItemInput{Name: "红烧肉", Category: "荤菜", Ingredients: "[]", Seasonings: "[]", Steps: `["煮"]`}
	if _, err := svc.SaveFromAI(in, service.CatalogSourceAISearch, "经典做法"); err != nil {
		t.Fatal(err)
	}

	result, err := svc.LookupOrGenerate(context.Background(), 1, "红烧肉", service.CatalogLookupOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Generated {
		t.Fatal("should hit cache without AI")
	}
	if len(result.Variants) != 1 || result.SelectedID == 0 {
		t.Fatalf("%+v", result)
	}
	if result.RateLimit == nil || result.RateLimit.Used != 0 {
		t.Fatalf("peek only, used should be 0: %+v", result.RateLimit)
	}
}

func TestCatalogLookupOrGenerateCallsAI(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)

	llmBody := `{"items":[{"name":"新菜A","category":"素菜","difficulty":"easy","cook_time":10,"ingredients":"[]","seasonings":"[]","steps":"[\"清炒\"]","tips":"","reason":""}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": llmBody}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	config.AppConfig = &config.Config{
		AI: config.AIConfig{
			APIKey:  "k",
			BaseURL: srv.URL,
			RateLimit: config.AIRateLimitConfig{
				Catalog: config.AIRateLimitScopeConfig{Enabled: true, MaxRequests: 5, WindowHours: 2},
			},
		},
	}
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	ai := service.NewAIServiceWithClient(srv.Client())
	service.SetAIServiceBaseURLForTest(ai, srv.URL)
	svc := service.NewCatalogRecipeService(db, ai, service.NewAIRateLimitService(store))

	result, err := svc.LookupOrGenerate(context.Background(), 9, "新菜A", service.CatalogLookupOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Generated || len(result.Variants) != 1 {
		t.Fatalf("%+v", result)
	}
	var count int64
	db.Model(&model.CatalogRecipe{}).Count(&count)
	if count != 1 {
		t.Fatalf("catalog count=%d", count)
	}
}

func TestCatalogNewVariantAddsSecondRow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)

	call := 0
	llmBodies := []string{
		`{"items":[{"name":"鱼香肉丝","category":"川菜","difficulty":"medium","cook_time":20,"ingredients":"[]","seasonings":"[]","steps":"[\"经典\"]","tips":"","reason":""}]}`,
		`{"items":[{"name":"鱼香肉丝","category":"湘菜","difficulty":"medium","cook_time":25,"ingredients":"[]","seasonings":"[]","steps":"[\"改良\"]","tips":"","reason":""}]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := llmBodies[call]
		if call < len(llmBodies)-1 {
			call++
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": body}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	config.AppConfig = &config.Config{
		AI: config.AIConfig{APIKey: "k", BaseURL: srv.URL, RateLimit: config.AIRateLimitConfig{
			Catalog: config.AIRateLimitScopeConfig{Enabled: true, MaxRequests: 5, WindowHours: 2},
		}},
	}
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	ai := service.NewAIServiceWithClient(srv.Client())
	service.SetAIServiceBaseURLForTest(ai, srv.URL)
	svc := service.NewCatalogRecipeService(db, ai, service.NewAIRateLimitService(store))

	if _, err := svc.LookupOrGenerate(context.Background(), 1, "鱼香肉丝", service.CatalogLookupOpts{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LookupOrGenerate(context.Background(), 1, "鱼香肉丝", service.CatalogLookupOpts{NewVariant: true}); err != nil {
		t.Fatal(err)
	}
	list, _ := svc.LookupByNameKey(service.NormalizeNameKey("鱼香肉丝"))
	if len(list) != 2 {
		t.Fatalf("want 2 variants got %d", len(list))
	}
}
