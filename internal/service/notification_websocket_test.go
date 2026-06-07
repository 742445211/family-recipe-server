package service

import (
	"testing"

	"recipe-server/internal/model"
)

func TestPendingWebSocketNotificationsExcludesSent(t *testing.T) {
	initTestConfig()
	requireNotificationEnabled(t)

	db := setupTestDB(t)
	orderID, chefID := seedChefOrder(t, db)

	svc := NewNotificationService(db, NewWebSocketHub())
	_ = svc.NotifyOrderCreated(orderID)
	waitNotificationAsync(t)

	var n model.Notification
	db.Where("order_id = ? AND receiver_user_id = ?", orderID, chefID).First(&n)
	db.Create(&model.NotificationDelivery{
		NotificationID: n.ID,
		Channel:        model.ChannelWebSocket,
		Status:         model.DeliveryStatusSent,
		Target:         "online",
	})

	pending, err := svc.pendingWebSocketNotifications(chefID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("已 WebSocket 送达的不应再补推, got %d", len(pending))
	}
}

func TestMarkWebSocketDeliverySentUpdatesSkipped(t *testing.T) {
	initTestConfig()

	db := setupTestDB(t)
	orderID, _ := seedChefOrder(t, db)

	n := model.Notification{
		FamilyID: 1, ReceiverUserID: 2, OrderID: orderID,
		Type: model.NotificationTypeOrderCreated, Title: "t", Content: "c",
		Status: model.NotificationStatusUnread,
	}
	db.Create(&n)
	d := model.NotificationDelivery{
		NotificationID: n.ID,
		Channel:        model.ChannelWebSocket,
		Status:         model.DeliveryStatusSkipped,
		Target:         "offline",
		ErrorMessage:   "user offline",
	}
	db.Create(&d)

	svc := NewNotificationService(db, NewWebSocketHub())
	svc.markWebSocketDeliverySent(n.ID)

	var updated model.NotificationDelivery
	db.First(&updated, d.ID)
	if updated.Status != model.DeliveryStatusSent {
		t.Fatalf("status: want sent, got %s", updated.Status)
	}
	if updated.Target != "online" {
		t.Fatalf("target: want online, got %s", updated.Target)
	}
}
