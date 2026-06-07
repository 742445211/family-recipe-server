package notifier

import (
	"context"
	"testing"
)

func TestServerChanMissingSendKey(t *testing.T) {
	n := NewServerChanNotifier(true)
	_, err := n.Send(context.Background(), NotificationMessage{Title: "有新的点菜"}, NotificationTarget{})
	if err == nil {
		t.Fatal("缺少 SendKey 应返回错误")
	}
}
