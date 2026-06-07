package service

import (
	"testing"
	"time"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

func seedChefOrder(t *testing.T, db *gorm.DB) (orderID uint64, chefID uint64) {
	t.Helper()

	adder := model.User{OpenID: "adder-oid", Nickname: "点菜人"}
	chef := model.User{OpenID: "chef-oid", Nickname: "厨师"}
	db.Create(&adder)
	db.Create(&chef)

	family := model.Family{Name: "测试家", InviteCode: "CHEF01"}
	db.Create(&family)
	db.Create(&model.FamilyMember{FamilyID: family.ID, UserID: adder.ID, Role: "member"})
	db.Create(&model.FamilyMember{FamilyID: family.ID, UserID: chef.ID, Role: "member", IsChef: true})

	recipe := model.Recipe{Name: "红烧肉", Category: "荤菜", CreatorID: adder.ID, FamilyID: family.ID}
	db.Create(&recipe)

	order := model.DailyOrder{
		FamilyID: family.ID, Date: "2026-06-05", MealType: "dinner",
		RecipeID: recipe.ID, AddedBy: adder.ID, Quantity: 1,
	}
	db.Create(&order)
	return order.ID, chef.ID
}

func waitNotificationAsync(t *testing.T) {
	t.Helper()
	time.Sleep(200 * time.Millisecond)
}

func TestNotifyOrderCreatedCreatesNotification(t *testing.T) {
	initTestConfig()
	requireNotificationEnabled(t)

	db := setupTestDB(t)
	orderID, chefID := seedChefOrder(t, db)
	hub := NewWebSocketHub()
	svc := NewNotificationService(db, hub)

	if err := svc.NotifyOrderCreated(orderID); err != nil {
		t.Fatalf("NotifyOrderCreated: %v", err)
	}

	var count int64
	db.Model(&model.Notification{}).Where("order_id = ? AND receiver_user_id = ?", orderID, chefID).Count(&count)
	if count != 1 {
		t.Errorf("通知数: want 1, got %d", count)
	}

	var n model.Notification
	db.Where("order_id = ? AND receiver_user_id = ?", orderID, chefID).First(&n)
	if n.Type != model.NotificationTypeOrderCreated {
		t.Errorf("type: want ORDER_CREATED, got %s", n.Type)
	}
	if n.Status != model.NotificationStatusUnread {
		t.Errorf("status: want unread, got %s", n.Status)
	}
	if n.Content == "" {
		t.Error("content 不应为空")
	}
}

func TestNotifyOrderCreatedIdempotent(t *testing.T) {
	initTestConfig()
	requireNotificationEnabled(t)

	db := setupTestDB(t)
	orderID, chefID := seedChefOrder(t, db)
	hub := NewWebSocketHub()
	svc := NewNotificationService(db, hub)

	_ = svc.NotifyOrderCreated(orderID)
	_ = svc.NotifyOrderCreated(orderID)
	waitNotificationAsync(t)

	var count int64
	db.Model(&model.Notification{}).Where("order_id = ? AND receiver_user_id = ?", orderID, chefID).Count(&count)
	if count != 1 {
		t.Errorf("幂等通知数: want 1, got %d", count)
	}
}

func TestNotifyOrderCreatedNoChef(t *testing.T) {
	initTestConfig()

	db := setupTestDB(t)
	userID, familyID := seedUserAndFamily(t, db)
	recipe := model.Recipe{Name: "番茄炒蛋", CreatorID: userID, FamilyID: familyID}
	db.Create(&recipe)
	order := model.DailyOrder{FamilyID: familyID, Date: "2026-06-05", MealType: "lunch", RecipeID: recipe.ID, AddedBy: userID}
	db.Create(&order)

	hub := NewWebSocketHub()
	svc := NewNotificationService(db, hub)
	if err := svc.NotifyOrderCreated(order.ID); err != nil {
		t.Fatalf("无厨师不应报错: %v", err)
	}
	var count int64
	db.Model(&model.Notification{}).Count(&count)
	if count != 0 {
		t.Errorf("无厨师通知数: want 0, got %d", count)
	}
}

func TestNotifyOrderCreatedSkipsWhenDisabled(t *testing.T) {
	initTestConfigNotification(false)

	db := setupTestDB(t)
	orderID, chefID := seedChefOrder(t, db)
	hub := NewWebSocketHub()
	svc := NewNotificationService(db, hub)

	if err := svc.NotifyOrderCreated(orderID); err != nil {
		t.Fatalf("NotifyOrderCreated: %v", err)
	}

	var count int64
	db.Model(&model.Notification{}).Where("order_id = ? AND receiver_user_id = ?", orderID, chefID).Count(&count)
	if count != 0 {
		t.Errorf("notification.enabled=false 时不应创建通知, got %d", count)
	}
}

func TestListUnreadAndMarkRead(t *testing.T) {
	initTestConfig()
	requireNotificationEnabled(t)

	db := setupTestDB(t)
	orderID, chefID := seedChefOrder(t, db)
	hub := NewWebSocketHub()
	svc := NewNotificationService(db, hub)
	_ = svc.NotifyOrderCreated(orderID)
	waitNotificationAsync(t)

	list, err := svc.ListUnread(chefID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("未读数: want 1, got %d", len(list))
	}

	if err := svc.MarkRead(chefID, list[0].ID); err != nil {
		t.Fatal(err)
	}
	list, _ = svc.ListUnread(chefID)
	if len(list) != 0 {
		t.Errorf("标记已读后未读数: want 0, got %d", len(list))
	}
}
