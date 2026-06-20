package service_test
import (
	"recipe-server/internal/service"
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestMenuCreateAddConfirm(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	recipe := model.Recipe{Name: "宫保鸡丁", CreatorID: userID, FamilyID: familyID}
	db.Create(&recipe)

	svc := service.NewMenuService(db)
	menu := &model.Menu{
		FamilyID: familyID, Name: "周末菜单", Date: "2026-06-08",
		CreatorID: userID, Status: "draft",
	}
	if err := svc.Create(menu); err != nil {
		t.Fatal(err)
	}

	item, err := svc.AddItem(menu.ID, recipe.ID, userID, 2, "微辣")
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if item.Quantity != 2 {
		t.Fatalf("quantity: got %d", item.Quantity)
	}

	got, err := svc.GetByID(menu.ID)
	if err != nil || len(got.Items) != 1 {
		t.Fatalf("GetByID items: %+v err=%v", got, err)
	}

	if err := svc.ConfirmMenu(menu.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddItem(menu.ID, recipe.ID, userID, 1, ""); err == nil {
		t.Fatal("已确认菜单不应允许加菜")
	}
}

func TestMenuRemoveItem(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	recipe := model.Recipe{Name: "鱼香肉丝", CreatorID: userID, FamilyID: familyID}
	db.Create(&recipe)

	svc := service.NewMenuService(db)
	menu := &model.Menu{FamilyID: familyID, Name: "工作日", Date: "2026-06-09", CreatorID: userID}
	db.Create(menu)
	item, _ := svc.AddItem(menu.ID, recipe.ID, userID, 1, "")

	if err := svc.RemoveItem(item.ID, userID); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetByID(menu.ID)
	if len(got.Items) != 0 {
		t.Fatalf("删除后 items 应为空, got %d", len(got.Items))
	}
}

func TestMenuAddItemMenuNotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := service.NewMenuService(db)
	_, err := svc.AddItem(9999, 1, 1, 1, "")
	if err == nil || err.Error() != "菜单不存在" {
		t.Fatalf("应返回菜单不存在, got %v", err)
	}
}

func TestMenuRemoveItemNotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := service.NewMenuService(db)
	err := svc.RemoveItem(9999, 1)
	if err == nil || err.Error() != "点菜项不存在" {
		t.Fatalf("应返回点菜项不存在, got %v", err)
	}
}

func TestMenuListPagination(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	svc := service.NewMenuService(db)
	for i := 0; i < 3; i++ {
		m := &model.Menu{FamilyID: familyID, Name: "菜单", Date: "2026-06-10", CreatorID: userID}
		if err := svc.Create(m); err != nil {
			t.Fatal(err)
		}
	}
	list, total, err := svc.List(familyID, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(list) != 2 {
		t.Fatalf("分页: total=%d len=%d", total, len(list))
	}
}

func TestMenuRemoveItemConfirmedMenu(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	recipe := model.Recipe{Name: "回锅肉", CreatorID: userID, FamilyID: familyID}
	db.Create(&recipe)
	svc := service.NewMenuService(db)
	menu := &model.Menu{FamilyID: familyID, Name: "确认菜单", Date: "2026-06-11", CreatorID: userID, Status: "confirmed"}
	db.Create(menu)
	item := model.MenuItem{MenuID: menu.ID, RecipeID: recipe.ID, AddedBy: userID, Quantity: 1}
	db.Create(&item)

	err := svc.RemoveItem(item.ID, userID)
	if err == nil || err.Error() != "菜单已确认，无法修改" {
		t.Fatalf("应拒绝修改已确认菜单, got %v", err)
	}
}
