package service_test
import (
	"recipe-server/internal/service"
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestRecipeCRUD(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_, familyID := testutil.SeedUserAndFamily(t, db)
	svc := service.NewRecipeService(db)

	// Create
	r := &model.Recipe{
		Name:       "番茄炒蛋",
		Category:   "荤菜",
		Difficulty: "easy",
		CookTime:   15,
		CreatorID:  1,
		FamilyID:   familyID,
		Ingredients: `[{"name":"番茄","amount":"2个"},{"name":"鸡蛋","amount":"3个"}]`,
		Steps:      `["番茄切块","鸡蛋打散","先炒蛋再加番茄"]`,
	}
	if err := svc.Create(r); err != nil {
		t.Fatalf("创建菜谱失败: %v", err)
	}
	if r.ID == 0 {
		t.Fatal("创建后 ID 应为非零")
	}

	// GetByID
	got, err := svc.GetByID(r.ID, familyID)
	if err != nil {
		t.Fatalf("获取菜谱失败: %v", err)
	}
	if got.Name != "番茄炒蛋" {
		t.Errorf("菜名: want 番茄炒蛋, got %s", got.Name)
	}

	// Update
	if err := svc.Update(&model.Recipe{ID: r.ID, Name: "番茄炒蛋升级版"}, familyID); err != nil {
		t.Fatalf("更新菜谱失败: %v", err)
	}
	got, _ = svc.GetByID(r.ID, familyID)
	if got.Name != "番茄炒蛋升级版" {
		t.Errorf("更新后菜名: want 番茄炒蛋升级版, got %s", got.Name)
	}

	// List
	list, total, err := svc.List(familyID, "", "", 1, 10)
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if total != 1 {
		t.Errorf("总数: want 1, got %d", total)
	}
	if len(list) != 1 {
		t.Errorf("列表长度: want 1, got %d", len(list))
	}

	// Search
	list, total, _ = svc.List(familyID, "番茄", "", 1, 10)
	if total != 1 {
		t.Errorf("搜索番茄: want 1, got %d", total)
	}
	list, total, _ = svc.List(familyID, "不存在", "", 1, 10)
	if total != 0 {
		t.Errorf("搜索不存在: want 0, got %d", total)
	}

	// CookCount
	svc.IncrementCookCount(r.ID, familyID)
	got, _ = svc.GetByID(r.ID, familyID)
	if got.CookCount != 1 {
		t.Errorf("做过次数: want 1, got %d", got.CookCount)
	}

	// Delete
	if err := svc.Delete(r.ID, 1, familyID); err != nil {
		t.Fatalf("删除菜谱失败: %v", err)
	}
	_, err = svc.GetByID(r.ID, familyID)
	if err == nil {
		t.Fatal("删除后仍能查到菜谱")
	}
}

func TestRecipeFamilyIsolation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_, familyA := testutil.SeedUserAndFamily(t, db)
	userB := model.User{OpenID: "user-b", Nickname: "用户B"}
	db.Create(&userB)
	familyB := model.Family{Name: "家庭B", InviteCode: "FAMBBB"}
	db.Create(&familyB)
	db.Create(&model.FamilyMember{FamilyID: familyB.ID, UserID: userB.ID, Role: "owner"})

	svc := service.NewRecipeService(db)
	r := &model.Recipe{Name: "家庭A专属菜", CreatorID: 1, FamilyID: familyA, IsPublic: false}
	if err := svc.Create(r); err != nil {
		t.Fatalf("创建菜谱失败: %v", err)
	}

	if _, err := svc.GetByID(r.ID, familyB.ID); err == nil {
		t.Fatal("其他家庭不应能按 ID 读到私有菜谱")
	}
	got, err := svc.GetByID(r.ID, familyA)
	if err != nil || got.Name != "家庭A专属菜" {
		t.Fatalf("本家庭应能读到菜谱: err=%v got=%+v", err, got)
	}

	list, total, err := svc.List(familyB.ID, "", "", 1, 10)
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("其他家庭列表不应包含私有菜谱, total=%d len=%d", total, len(list))
	}

	list, total, err = svc.List(0, "", "", 1, 10)
	if err != nil {
		t.Fatalf("familyID=0 列表查询失败: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("未登录时不应返回私有菜谱, total=%d len=%d", total, len(list))
	}
}

func TestRecipePublicVisibleToAll(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_, familyA := testutil.SeedUserAndFamily(t, db)
	familyB := model.Family{Name: "家庭B", InviteCode: "FAMBBB"}
	db.Create(&familyB)

	svc := service.NewRecipeService(db)
	pub := &model.Recipe{Name: "公开菜", CreatorID: 1, FamilyID: familyA, IsPublic: true}
	if err := svc.Create(pub); err != nil {
		t.Fatalf("创建公开菜谱失败: %v", err)
	}

	got, err := svc.GetByID(pub.ID, familyB.ID)
	if err != nil || got.Name != "公开菜" {
		t.Fatalf("其他家庭应能读到公开菜谱: err=%v", err)
	}
	got, err = svc.GetByID(pub.ID, 0)
	if err != nil || got.Name != "公开菜" {
		t.Fatalf("未指定家庭应能读到公开菜谱: err=%v", err)
	}

	list, total, err := svc.List(familyB.ID, "", "", 1, 10)
	if err != nil || total != 1 || len(list) != 1 || list[0].Name != "公开菜" {
		t.Fatalf("列表应包含公开菜谱: total=%d list=%+v err=%v", total, list, err)
	}
	list, total, err = svc.List(0, "", "", 1, 10)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("未登录列表应只含公开菜谱: total=%d len=%d err=%v", total, len(list), err)
	}
}
