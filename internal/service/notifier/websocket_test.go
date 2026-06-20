package notifier

import "testing"

func TestBuildOrderCreatedPayloadIncludesNotificationID(t *testing.T) {
	payload := BuildOrderCreatedPayload(NotificationMessage{
		NotificationID: 42,
		Title:        "有新的点菜",
		Content:      "晚餐新增：红烧肉",
		OrderID:      7,
		Date:         "2026-06-05",
		MealType:     "dinner",
	})
	if payload["type"] != "ORDER_CREATED" {
		t.Fatalf("type: %v", payload["type"])
	}
	if payload["notification_id"] != uint64(42) {
		t.Fatalf("notification_id: %v", payload["notification_id"])
	}
}
