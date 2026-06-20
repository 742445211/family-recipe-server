package service

import (
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestCategoryEnsureCreatesOnce(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_, fid := testutil.SeedUserAndFamily(t, db)
	svc := NewCategoryService(db)

	name, err := svc.Ensure(fid, "川菜")
	if err != nil || name != "川菜" {
		t.Fatalf("Ensure: name=%q err=%v", name, err)
	}
	name2, err := svc.Ensure(fid, " 川菜 ")
	if err != nil || name2 != "川菜" {
		t.Fatalf("应归一化并复用: %q err=%v", name2, err)
	}
	var count int64
	db.Model(&model.RecipeCategory{}).Where("family_id = ? AND name = ?", fid, "川菜").Count(&count)
	if count != 1 {
		t.Fatalf("应只有一条分类记录, got %d", count)
	}
}

func TestCategoryEnsureEmptyDefaultsToOther(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_, fid := testutil.SeedUserAndFamily(t, db)
	name, err := NewCategoryService(db).Ensure(fid, "")
	if err != nil || name != "其他" {
		t.Fatalf("空分类应归一为其他: %q err=%v", name, err)
	}
}

func TestCategorySyncFromRecipes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid, fid := testutil.SeedUserAndFamily(t, db)
	db.Create(&model.Recipe{Name: "红烧肉", Category: "荤菜", CreatorID: uid, FamilyID: fid, IsPublic: false})
	db.Create(&model.Recipe{Name: "炒青菜", Category: "素菜", CreatorID: uid, FamilyID: fid, IsPublic: false})
	db.Create(&model.Recipe{Name: "无分类", Category: "", CreatorID: uid, FamilyID: fid, IsPublic: false})

	svc := NewCategoryService(db)
	if err := svc.SyncFromRecipes(fid); err != nil {
		t.Fatal(err)
	}
	names, err := svc.ListNames(fid)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"荤菜": true, "素菜": true, "其他": true}
	if len(names) != len(want) {
		t.Fatalf("分类数量: got %v", names)
	}
	for _, n := range names {
		if !want[n] {
			t.Fatalf("意外分类 %q in %v", n, names)
		}
	}
}

func TestCategoryListPublicNames(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid, fid := testutil.SeedUserAndFamily(t, db)
	other := model.Family{Name: "其他家", InviteCode: "PUB001"}
	db.Create(&other)

	recipeSvc := NewRecipeService(db)
	create := func(name, cat string, fid uint64, pub bool) {
		r := &model.Recipe{Name: name, Category: cat, CreatorID: uid, FamilyID: fid, IsPublic: pub}
		if err := recipeSvc.Create(r); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	create("公开荤菜", "荤菜", fid, true)
	create("公开汤", "汤", other.ID, true)
	create("私有素菜", "素菜", fid, false)
	create("空分类公开", "", fid, true)

	names, err := NewCategoryService(db).ListPublicNames()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"其他", "汤", "荤菜"}
	if len(names) != len(want) {
		t.Fatalf("公开分类: got %v want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Fatalf("公开分类顺序/内容: got %v want %v", names, want)
		}
	}
}
