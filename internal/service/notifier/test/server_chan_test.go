package notifier_test
import (
	"recipe-server/internal/service/notifier"
	"context"
	"testing"
)

func TestServerChanMissingSendKey(t *testing.T) {
	n := notifier.NewServerChanNotifier(true)
	_, err := n.Send(context.Background(), notifier.NotificationMessage{Title: "有新的点菜"}, notifier.NotificationTarget{})
	if err == nil {
		t.Fatal("缺少 SendKey 应返回错误")
	}
}
