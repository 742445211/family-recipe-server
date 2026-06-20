package notifier

import (
	"context"
	"testing"
)

func TestNtfyMissingTopic(t *testing.T) {
	n := NewNtfyNotifier(true)
	_, err := n.Send(context.Background(), NotificationMessage{Title: "t", Content: "c"}, NotificationTarget{})
	if err == nil {
		t.Fatal("缺少 topic 应返回错误")
	}
}
