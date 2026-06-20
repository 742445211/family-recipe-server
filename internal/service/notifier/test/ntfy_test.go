package notifier_test
import (
	"recipe-server/internal/service/notifier"
	"context"
	"testing"
)

func TestNtfyMissingTopic(t *testing.T) {
	n := notifier.NewNtfyNotifier(true)
	_, err := n.Send(context.Background(), notifier.NotificationMessage{Title: "t", Content: "c"}, notifier.NotificationTarget{})
	if err == nil {
		t.Fatal("缺少 topic 应返回错误")
	}
}
