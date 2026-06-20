package notifier

import (
	"context"
	"testing"
)

func TestBarkMissingDeviceKey(t *testing.T) {
	n := NewBarkNotifier(true)
	_, err := n.Send(context.Background(), NotificationMessage{Title: "t", Content: "c"}, NotificationTarget{})
	if err == nil {
		t.Fatal("缺少 device key 应返回错误")
	}
}
