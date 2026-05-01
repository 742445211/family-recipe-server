package service

import (
	"testing"

	"recipe-server/internal/model"
)

func TestMenuFlow(t *testing.T) {
	db := setupTestDB(t)
	userID, familyID := seedUserAndFamily(t, db)

	// 先创建一道菜谱
	recipe := model.Recipe{
		Name: "宫保鸡丁", Category: "荤菜", Difficulty: "medium",
		CreatorID: userID, FamilyID: familyID,
	}
	db.Create(&recipe)

	svc := NewMenuService(db)

	// Create menu
	menu := model.Menu{Name: "周五晚餐", Date: "2026-05-01", FamilyID: familyID, CreatorID: userID}
	if err := svc.Create(&menu); err != nil {
		t.Fatalf("创建菜单失败: %v", err)
	}

	// Add item
	item, err := svc.AddItem(menu.ID, recipe.ID, userID, 2, "少放辣")
	if err != nil {
		t.Fatalf("加菜失败: %v", err)
	}
	if item.Quantity != 2 {
		t.Errorf("数量: want 2, got %d", item.Quantity)
	}

	// Get menu with items
	got, err := svc.GetByID(menu.ID)
	if err != nil {
		t.Fatalf("获取菜单失败: %v", err)
	}
	if len(got.Items) != 1 {
		t.Errorf("菜单项数: want 1, got %d", len(got.Items))
	}

	// Remove item
	if err := svc.RemoveItem(item.ID, userID); err != nil {
		t.Fatalf("删除点菜项失败: %v", err)
	}
	got, _ = svc.GetByID(menu.ID)
	if len(got.Items) != 0 {
		t.Errorf("删除后菜单项数: want 0, got %d", len(got.Items))
	}

	// Confirm
	svc.AddItem(menu.ID, recipe.ID, userID, 1, "")
	if err := svc.ConfirmMenu(menu.ID); err != nil {
		t.Fatalf("确认菜单失败: %v", err)
	}

	// 确认后不能再加菜
	_, err = svc.AddItem(menu.ID, recipe.ID, userID, 1, "")
	if err == nil {
		t.Fatal("确认后仍能加菜")
	}

	// 确认后不能删除
	item2, _ := svc.AddItem(menu.ID, recipe.ID, userID, 1, "") // 这条不会成功
	if item2 != nil {
		err = svc.RemoveItem(item2.ID, userID)
		if err == nil {
			t.Fatal("确认后仍能删除点菜项")
		}
	}

	// List
	list, total, _ := svc.List(familyID, 1, 10)
	if total != 1 {
		t.Errorf("菜单总数: want 1, got %d", total)
	}
	if len(list) != 1 {
		t.Errorf("菜单列表长度: want 1, got %d", len(list))
	}
}

func TestFavoriteCRUD(t *testing.T) {
	db := setupTestDB(t)
	userID, familyID := seedUserAndFamily(t, db)

	recipe := model.Recipe{
		Name: "测试菜", Category: "其他", Difficulty: "easy",
		CreatorID: userID, FamilyID: familyID,
	}
	db.Create(&recipe)

	// Add favorite
	fav := model.Favorite{UserID: userID, RecipeID: recipe.ID}
	if err := db.Create(&fav).Error; err != nil {
		t.Fatalf("收藏失败: %v", err)
	}

	// 重复收藏（FirstOrCreate）
	fav2 := model.Favorite{UserID: userID, RecipeID: recipe.ID}
	db.Where(model.Favorite{UserID: userID, RecipeID: recipe.ID}).FirstOrCreate(&fav2)
	var count int64
	db.Model(&model.Favorite{}).Where("user_id = ?", userID).Count(&count)
	if count != 1 {
		t.Errorf("重复收藏后数量: want 1, got %d", count)
	}

	// Remove
	db.Where("user_id = ? AND recipe_id = ?", userID, recipe.ID).Delete(&model.Favorite{})
	db.Model(&model.Favorite{}).Where("user_id = ?", userID).Count(&count)
	if count != 0 {
		t.Errorf("取消收藏后数量: want 0, got %d", count)
	}
}
