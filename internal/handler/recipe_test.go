package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/service"
	"recipe-server/internal/testutil"
	jwtPkg "recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
)

func setupRecipeRouter(t *testing.T) (*gin.Engine, *RecipeHandler, uint64, uint64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	testutil.EnsureAppConfig()
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	otherFamily := model.Family{Name: "其他家庭", InviteCode: "OTHER2"}
	db.Create(&otherFamily)
	seedRecipe := func(name string, fid uint64, pub bool) {
		r := model.Recipe{Name: name, CreatorID: userID, FamilyID: fid, IsPublic: pub}
		if err := service.NewRecipeService(db).Create(&r); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	seedRecipe("外来私有菜谱", otherFamily.ID, false)
	seedRecipe("外来公开菜谱", otherFamily.ID, true)
	seedRecipe("本家菜谱", familyID, false)

	h := NewRecipeHandler(db)
	r := gin.New()
	pub := r.Group("/api").Use(middleware.OptionalAuth())
	pub.GET("/recipes", h.List)
	pub.GET("/recipes/:id", h.Get)
	return r, h, userID, familyID
}

func authRequest(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestRecipeListPublicWithoutAuth(t *testing.T) {
	r, _, _, _ := setupRecipeRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/recipes", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("未登录应能浏览公开菜谱, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			List     []model.Recipe `json:"list"`
			Total    int64          `json:"total"`
			HasMore  bool           `json:"has_more"`
			Page     int            `json:"page"`
			PageSize int            `json:"page_size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Total != 1 || len(resp.Data.List) != 1 || resp.Data.List[0].Name != "外来公开菜谱" {
		t.Fatalf("未登录应只返回公开菜谱: %+v", resp.Data)
	}
	if resp.Data.HasMore {
		t.Fatalf("单条结果 has_more 应为 false")
	}
}

func TestRecipeListPagination(t *testing.T) {
	r, _, userID, familyID := setupRecipeRouter(t)
	token, _ := jwtPkg.Generate(config.AppConfig.JWT.Secret, 24, userID, "oid", familyID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest(http.MethodGet, "/api/recipes?page=1&page_size=1", token))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var page1 struct {
		Data struct {
			List     []model.Recipe `json:"list"`
			Total    int64          `json:"total"`
			HasMore  bool           `json:"has_more"`
			Page     int            `json:"page"`
			PageSize int            `json:"page_size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page1); err != nil {
		t.Fatal(err)
	}
	if page1.Data.Total != 2 || len(page1.Data.List) != 1 || !page1.Data.HasMore {
		t.Fatalf("第一页分页异常: %+v", page1.Data)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest(http.MethodGet, "/api/recipes?page=2&page_size=1", token))
	if w.Code != http.StatusOK {
		t.Fatalf("page2 status %d", w.Code)
	}
	var page2 struct {
		Data struct {
			List    []model.Recipe `json:"list"`
			HasMore bool           `json:"has_more"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Data.List) != 1 || page2.Data.HasMore {
		t.Fatalf("第二页分页异常: %+v", page2.Data)
	}
}

func TestRecipeListIncludesOwnAndPublic(t *testing.T) {
	r, _, userID, familyID := setupRecipeRouter(t)
	token, _ := jwtPkg.Generate(config.AppConfig.JWT.Secret, 24, userID, "oid", familyID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest(http.MethodGet, "/api/recipes", token))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			List  []model.Recipe `json:"list"`
			Total int64          `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Total != 2 || len(resp.Data.List) != 2 {
		t.Fatalf("登录后应返回本家菜谱 + 公开菜谱: %+v", resp.Data)
	}
}

func TestRecipeGetRejectsOtherFamilyPrivate(t *testing.T) {
	r, h, userID, familyID := setupRecipeRouter(t)
	var privateRecipe model.Recipe
	h.db.Where("name = ?", "外来私有菜谱").First(&privateRecipe)

	token, _ := jwtPkg.Generate(config.AppConfig.JWT.Secret, 24, userID, "oid", familyID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest(http.MethodGet, "/api/recipes/"+strconv.FormatUint(privateRecipe.ID, 10), token))
	if w.Code != http.StatusNotFound {
		t.Fatalf("其他家庭私有菜谱应 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRecipeGetAllowsPublicRecipe(t *testing.T) {
	r, h, userID, familyID := setupRecipeRouter(t)
	var publicRecipe model.Recipe
	h.db.Where("name = ?", "外来公开菜谱").First(&publicRecipe)

	token, _ := jwtPkg.Generate(config.AppConfig.JWT.Secret, 24, userID, "oid", familyID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest(http.MethodGet, "/api/recipes/"+strconv.FormatUint(publicRecipe.ID, 10), token))
	if w.Code != http.StatusOK {
		t.Fatalf("公开菜谱应可读, got %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/recipes/"+strconv.FormatUint(publicRecipe.ID, 10), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("未登录也应能读公开菜谱, got %d", w.Code)
	}
}
