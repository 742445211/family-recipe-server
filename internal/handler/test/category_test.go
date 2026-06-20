package handler_test
import (
	"recipe-server/internal/handler"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/service"
	"recipe-server/internal/testutil"

	"github.com/gin-gonic/gin"
)

func setupCategoryRouter(t *testing.T) (*gin.Engine, uint64, uint64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	uid, fid := testutil.SeedUserAndFamily(t, db)
	other := model.Family{Name: "其他家", InviteCode: "PUBCAT"}
	db.Create(&other)
	recipeSvc := service.NewRecipeService(db)
	seed := func(name, cat string, familyID uint64, pub bool) {
		r := &model.Recipe{Name: name, Category: cat, CreatorID: uid, FamilyID: familyID, IsPublic: pub}
		if err := recipeSvc.Create(r); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	seed("公开A", "荤菜", fid, true)
	seed("私有B", "素菜", fid, false)
	seed("公开C", "汤", other.ID, true)

	h := handler.NewCategoryHandler(db)
	r := gin.New()
	r.GET("/api/categories/public", h.ListPublic)
	return r, uid, fid
}

func TestCategoryListPublicWithoutAuth(t *testing.T) {
	r, _, _ := setupCategoryRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/categories/public", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("未登录应能获取公开分类, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int      `json:"code"`
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 {
		t.Fatalf("code=%d", resp.Code)
	}
	want := map[string]bool{"荤菜": true, "汤": true}
	if len(resp.Data) != len(want) {
		t.Fatalf("公开分类: got %v", resp.Data)
	}
	for _, n := range resp.Data {
		if !want[n] {
			t.Fatalf("意外分类 %q in %v", n, resp.Data)
		}
	}
}
