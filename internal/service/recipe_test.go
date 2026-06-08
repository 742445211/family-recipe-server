package service

import (
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestRecipeCRUD(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_, familyID := testutil.SeedUserAndFamily(t, db)
	svc := NewRecipeService(db)

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
	got, err := svc.GetByID(r.ID)
	if err != nil {
		t.Fatalf("获取菜谱失败: %v", err)
	}
	if got.Name != "番茄炒蛋" {
		t.Errorf("菜名: want 番茄炒蛋, got %s", got.Name)
	}

	// Update
	if err := svc.Update(&model.Recipe{ID: r.ID, Name: "番茄炒蛋升级版"}); err != nil {
		t.Fatalf("更新菜谱失败: %v", err)
	}
	got, _ = svc.GetByID(r.ID)
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
	svc.IncrementCookCount(r.ID)
	got, _ = svc.GetByID(r.ID)
	if got.CookCount != 1 {
		t.Errorf("做过次数: want 1, got %d", got.CookCount)
	}

	// Delete
	if err := svc.Delete(r.ID, 1); err != nil {
		t.Fatalf("删除菜谱失败: %v", err)
	}
	_, err = svc.GetByID(r.ID)
	if err == nil {
		t.Fatal("删除后仍能查到菜谱")
	}
}
