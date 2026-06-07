package service

import (
	"testing"

	"recipe-server/internal/model"
)

func TestOrderAddAndList(t *testing.T) {
	db := setupTestDB(t)
	userID, familyID := seedUserAndFamily(t, db)

	recipe := model.Recipe{
		Name: "麻婆豆腐", Category: "荤菜", Difficulty: "medium",
		CreatorID: userID, FamilyID: familyID,
	}
	db.Create(&recipe)

	svc := NewOrderService(db)

	// Add
	order, err := svc.Add(familyID, recipe.ID, "dinner", userID, "2026-05-01", "少放花椒", 2)
	if err != nil {
		t.Fatalf("点菜失败: %v", err)
	}
	if order.Quantity != 2 {
		t.Errorf("数量: want 2, got %d", order.Quantity)
	}

	// 同餐次重复点菜应失败
	recipe2 := model.Recipe{Name: "宫保鸡丁", Category: "荤菜", CreatorID: userID, FamilyID: familyID}
	db.Create(&recipe2)
	_, err = svc.Add(familyID, recipe2.ID, "dinner", userID, "2026-05-01", "", 1)
	if err != nil {
		t.Fatalf("第二道菜点菜失败: %v", err)
	}

	// Get by date
	orders, err := svc.GetByDateAndMeal(familyID, "2026-05-01", "")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(orders) != 2 {
		t.Errorf("点菜数: want 2, got %d", len(orders))
	}

	// Different date should be empty
	orders, _ = svc.GetByDateAndMeal(familyID, "2026-05-02", "")
	if len(orders) != 0 {
		t.Errorf("5月2日应无点菜, got %d", len(orders))
	}

	// Remove
	if err := svc.Remove(order.ID, userID); err != nil {
		t.Fatalf("取消点菜失败: %v", err)
	}
	orders, _ = svc.GetByDateAndMeal(familyID, "2026-05-01", "")
	if len(orders) != 1 {
		t.Errorf("取消后数量: want 1, got %d", len(orders))
	}

	// 不能删除别人的点菜
	if err := svc.Remove(orders[0].ID, 999); err == nil {
		t.Fatal("删除他人点菜应失败")
	}
}
