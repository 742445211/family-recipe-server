package service_test
import (
	"recipe-server/internal/service"
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestOrderAddAndList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	recipe := model.Recipe{
		Name: "麻婆豆腐", Category: "荤菜", Difficulty: "medium",
		CreatorID: userID, FamilyID: familyID,
	}
	db.Create(&recipe)

	svc := service.NewOrderService(db)

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

func TestOrderAddRejectsOtherFamilyRecipe(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	otherFamily := model.Family{Name: "其他家", InviteCode: "OTHER1"}
	db.Create(&otherFamily)
	otherRecipe := model.Recipe{Name: "外来私有菜", CreatorID: userID, FamilyID: otherFamily.ID, IsPublic: false}
	if err := service.NewRecipeService(db).Create(&otherRecipe); err != nil {
		t.Fatalf("seed recipe: %v", err)
	}

	svc := service.NewOrderService(db)
	if _, err := svc.Add(familyID, otherRecipe.ID, "dinner", userID, "2026-05-01", "", 1); err == nil {
		t.Fatal("不应允许用其他家庭的私有菜谱点菜")
	}
}

func TestOrderAddAllowsPublicRecipeFromOtherFamily(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	otherFamily := model.Family{Name: "其他家", InviteCode: "OTHER2"}
	db.Create(&otherFamily)
	pub := model.Recipe{Name: "外来公开菜", CreatorID: userID, FamilyID: otherFamily.ID, IsPublic: true}
	if err := service.NewRecipeService(db).Create(&pub); err != nil {
		t.Fatalf("seed recipe: %v", err)
	}

	svc := service.NewOrderService(db)
	order, err := svc.Add(familyID, pub.ID, "dinner", userID, "2026-05-02", "", 1)
	if err != nil {
		t.Fatalf("应允许用公开菜谱点菜: %v", err)
	}
	if order.RecipeID != pub.ID {
		t.Fatalf("recipe_id: got %d", order.RecipeID)
	}
}
