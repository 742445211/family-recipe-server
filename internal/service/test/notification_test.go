package service_test
import (
	"recipe-server/internal/service"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"recipe-server/config"
	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestNotifyOrderCreatedCreatesNotification(t *testing.T) {
	testutil.InitTestConfig()
	testutil.RequireNotificationEnabled(t)

	db := testutil.SetupTestDB(t)
	orderID, chefID := testutil.SeedChefOrder(t, db)
	hub := service.NewWebSocketHub()
	svc := service.NewNotificationService(db, hub)

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
	testutil.InitTestConfig()
	testutil.RequireNotificationEnabled(t)

	db := testutil.SetupTestDB(t)
	orderID, chefID := testutil.SeedChefOrder(t, db)
	hub := service.NewWebSocketHub()
	svc := service.NewNotificationService(db, hub)

	_ = svc.NotifyOrderCreated(orderID)
	_ = svc.NotifyOrderCreated(orderID)
	service.WaitNotificationAsyncForTest(t)

	var count int64
	db.Model(&model.Notification{}).Where("order_id = ? AND receiver_user_id = ?", orderID, chefID).Count(&count)
	if count != 1 {
		t.Errorf("幂等通知数: want 1, got %d", count)
	}
}

func TestNotifyOrderCreatedNoChef(t *testing.T) {
	testutil.InitTestConfig()

	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	recipe := model.Recipe{Name: "番茄炒蛋", CreatorID: userID, FamilyID: familyID}
	db.Create(&recipe)
	order := model.DailyOrder{FamilyID: familyID, Date: "2026-06-05", MealType: "lunch", RecipeID: recipe.ID, AddedBy: userID}
	db.Create(&order)

	hub := service.NewWebSocketHub()
	svc := service.NewNotificationService(db, hub)
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
	testutil.InitTestConfigNotification(false)

	db := testutil.SetupTestDB(t)
	orderID, chefID := testutil.SeedChefOrder(t, db)
	hub := service.NewWebSocketHub()
	svc := service.NewNotificationService(db, hub)

	if err := svc.NotifyOrderCreated(orderID); err != nil {
		t.Fatalf("NotifyOrderCreated: %v", err)
	}

	var count int64
	db.Model(&model.Notification{}).Where("order_id = ? AND receiver_user_id = ?", orderID, chefID).Count(&count)
	if count != 0 {
		t.Errorf("notification.enabled=false 时不应创建通知, got %d", count)
	}
}

func TestNotifyOrderCreatedPushesEachChefWecom(t *testing.T) {
	var mu sync.Mutex
	var tousers []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/gettoken" {
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
			return
		}
		if r.URL.Path == "/cgi-bin/message/send" {
			b, _ := io.ReadAll(r.Body)
			var payload struct {
				ToUser string `json:"touser"`
			}
			_ = json.Unmarshal(b, &payload)
			mu.Lock()
			tousers = append(tousers, payload.ToUser)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
			return
		}
		_, _ = w.Write([]byte(`{"errcode":0}`))
	}))
	t.Cleanup(srv.Close)

	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			Enabled: true,
			Retry:   config.NotificationRetry{MaxAttempts: 3, IntervalsSec: []int{60, 300, 900}},
			WecomWorkbench: config.NotificationWecom{
				Enabled: true, CorpID: "c", AgentID: 1, Secret: "s",
				APIBase: srv.URL, MsgType: "text", DuplicateCheckInterval: 1800,
			},
		},
	}

	db := testutil.SetupTestDB(t)
	adder := model.User{OpenID: "adder-x", Nickname: "点菜人"}
	chefA := model.User{OpenID: "chefA", Nickname: "厨师A"}
	chefB := model.User{OpenID: "chefB", Nickname: "厨师B"}
	db.Create(&adder)
	db.Create(&chefA)
	db.Create(&chefB)
	family := model.Family{Name: "多厨之家", InviteCode: "MULTI1"}
	db.Create(&family)
	db.Create(&model.FamilyMember{FamilyID: family.ID, UserID: adder.ID, Role: "member"})
	db.Create(&model.FamilyMember{FamilyID: family.ID, UserID: chefA.ID, Role: "member", IsChef: true})
	db.Create(&model.FamilyMember{FamilyID: family.ID, UserID: chefB.ID, Role: "member", IsChef: true})
	db.Create(&model.NotificationChannel{UserID: chefA.ID, Channel: model.ChannelWecomWorkbench, Enabled: true, Secret: "useridA"})
	db.Create(&model.NotificationChannel{UserID: chefB.ID, Channel: model.ChannelWecomWorkbench, Enabled: true, Secret: "useridB"})
	recipe := model.Recipe{Name: "红烧肉", Category: "荤菜", CreatorID: adder.ID, FamilyID: family.ID}
	db.Create(&recipe)
	order := model.DailyOrder{FamilyID: family.ID, Date: "2026-06-05", MealType: "dinner", RecipeID: recipe.ID, AddedBy: adder.ID, Quantity: 1}
	db.Create(&order)

	svc := service.NewNotificationService(db, service.NewWebSocketHub())
	if err := svc.NotifyOrderCreated(order.ID); err != nil {
		t.Fatalf("NotifyOrderCreated: %v", err)
	}

	var got []string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got = append([]string(nil), tousers...)
		mu.Unlock()
		if len(got) >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	sort.Strings(got)
	if len(got) != 2 || got[0] != "useridA" || got[1] != "useridB" {
		t.Fatalf("应给每个厨师各推送一次企微，got=%v", got)
	}

	var nCount int64
	db.Model(&model.Notification{}).Where("order_id = ?", order.ID).Count(&nCount)
	if nCount != 2 {
		t.Fatalf("应为每个厨师创建通知，got=%d", nCount)
	}
}

func TestListUnreadAndMarkRead(t *testing.T) {
	testutil.InitTestConfig()
	testutil.RequireNotificationEnabled(t)

	db := testutil.SetupTestDB(t)
	orderID, chefID := testutil.SeedChefOrder(t, db)
	hub := service.NewWebSocketHub()
	svc := service.NewNotificationService(db, hub)
	_ = svc.NotifyOrderCreated(orderID)
	service.WaitNotificationAsyncForTest(t)

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
