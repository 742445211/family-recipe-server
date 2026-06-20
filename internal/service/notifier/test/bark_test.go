package notifier_test
import (
	"recipe-server/internal/service/notifier"
	"context"
	"testing"
)

func TestBarkMissingDeviceKey(t *testing.T) {
	n := notifier.NewBarkNotifier(true)
	_, err := n.Send(context.Background(), notifier.NotificationMessage{Title: "t", Content: "c"}, notifier.NotificationTarget{})
	if err == nil {
		t.Fatal("缺少 device key 应返回错误")
	}
}
