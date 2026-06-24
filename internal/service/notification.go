// Package service - 厨师点菜通知服务。
//
// 点菜成功后：先写入 notifications 表（强一致），再异步调度各通知通道
// （WebSocket / 微信订阅 / 企微 / Server酱 / Bark / ntfy），投递记录写入 notification_deliveries。
// 离线 WebSocket 用户上线时通过 FlushUnreadWebSocket 补推；失败投递由 RetryWorker 重试。
package service

import (
	"context"
	"log"
	"strings"
	"time"

	"recipe-server/config"
	"recipe-server/internal/model"
	"recipe-server/internal/service/notifier"
	"recipe-server/internal/service/wechattoken"
	"recipe-server/pkg/dateutil"

	"gorm.io/gorm"
)

// NotificationService 厨师点菜通知服务。
type NotificationService struct {
	db        *gorm.DB
	hub       *WebSocketHub
	notifiers []notifier.Notifier
}

// NewNotificationService 创建通知服务。
func NewNotificationService(db *gorm.DB, hub *WebSocketHub) *NotificationService {
	cfg := config.AppConfig.Notification
	mpToken := wechattoken.SharedMiniProgramToken()
	wecomToken := wechattoken.NewWecomToken()

	n := &NotificationService{
		db:  db,
		hub: hub,
		notifiers: []notifier.Notifier{
			notifier.NewWebSocketNotifier(cfg.WebSocket.Enabled, hub),
			notifier.NewWeChatSubscribeNotifier(cfg.WeChatSubscribe.Enabled, mpToken),
			notifier.NewWecomWorkbenchNotifier(cfg.WecomWorkbench.Enabled, wecomToken),
			notifier.NewServerChanNotifier(cfg.ServerChan.Enabled),
			notifier.NewBarkNotifier(cfg.Bark.Enabled),
			notifier.NewNtfyNotifier(cfg.Ntfy.Enabled),
		},
	}
	return n
}

// NotifyOrderCreatedAsync 在后台异步执行点菜通知（落库与各通道投递均不阻塞调用方）。
func (s *NotificationService) NotifyOrderCreatedAsync(orderID uint64) {
	go func() {
		if err := s.NotifyOrderCreated(orderID); err != nil {
			log.Printf("[通知] 创建点菜通知失败 orderID=%d: %v", orderID, err)
		}
	}()
}

// NotifyOrderCreated 点菜成功后创建通知并调度各通道。
func (s *NotificationService) NotifyOrderCreated(orderID uint64) error {
	if config.AppConfig == nil || !config.AppConfig.Notification.Enabled {
		return nil
	}

	var order model.DailyOrder
	if err := s.db.Preload("Recipe").Preload("Adder").First(&order, orderID).Error; err != nil {
		return err
	}

	var chefs []model.FamilyMember
	s.db.Where("family_id = ? AND is_chef = ?", order.FamilyID, true).
		Preload("User").Find(&chefs)

	if len(chefs) == 0 {
		return nil
	}

	recipeName := ""
	ingredients := ""
	recipeCoverURL := ""
	if order.Recipe != nil {
		recipeName = order.Recipe.Name
		ingredients = order.Recipe.Ingredients
		recipeCoverURL = order.Recipe.CoverURL
	}
	adderName := ""
	if order.Adder != nil {
		adderName = order.Adder.Nickname
	}

	title := "有新的点菜"
	orderDate := dateutil.FormatYMD(order.Date)
	msgBase := notifier.NotificationMessage{
		RecipeName:     recipeName,
		AdderName:      adderName,
		MealType:       order.MealType,
		Date:           orderDate,
		Note:           order.Note,
		Ingredients:    ingredients,
		RecipeCoverURL: recipeCoverURL,
	}
	content := notifier.BuildOrderContent(msgBase)

	chSvc := NewNotificationChannelService(s.db)

	for _, chef := range chefs {
		if chef.User == nil {
			continue
		}
		n, err := s.ensureNotification(order, chef.UserID, title, content)
		if err != nil {
			log.Printf("[通知] 创建失败 chef=%d: %v", chef.UserID, err)
			continue
		}

		msg := notifier.NotificationMessage{
			NotificationID: n.ID,
			ReceiverUserID: chef.UserID,
			Title:          title,
			Content:        content,
			OrderID:        order.ID,
			RecipeName:     recipeName,
			AdderName:      adderName,
			MealType:       order.MealType,
			Date:           orderDate,
			OpenID:         chef.User.OpenID,
			Note:           order.Note,
			Ingredients:    ingredients,
			RecipeCoverURL: recipeCoverURL,
		}

		targets := chSvc.GetEnabledTargets(chef.UserID, chef.User)
		go s.dispatchNotification(n.ID, msg, targets)
	}
	return nil
}

func (s *NotificationService) ensureNotification(order model.DailyOrder, receiverID uint64, title, content string) (*model.Notification, error) {
	// 同一 order + 厨师 仅创建一条通知，避免重复点菜或重试时重复落库
	var existing model.Notification
	err := s.db.Where("order_id = ? AND receiver_user_id = ?", order.ID, receiverID).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	n := model.Notification{
		FamilyID:       order.FamilyID,
		ReceiverUserID: receiverID,
		OrderID:        order.ID,
		Type:           model.NotificationTypeOrderCreated,
		Title:          title,
		Content:        content,
		Status:         model.NotificationStatusUnread,
	}
	if err := s.db.Create(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// shouldDispatchChannel 判断是否向某通道发起投递。
// WebSocket 始终尝试（在线即推）；微信订阅在无用户通道配置时仍可用 OpenID 兜底。
func shouldDispatchChannel(ch string, n notifier.Notifier, targets map[string]notifierTarget, msg notifier.NotificationMessage) bool {
	if !n.Enabled() {
		return false
	}
	switch ch {
	case model.ChannelWebSocket:
		return true
	case model.ChannelWeChatSubscribe:
		return HasUserChannel(targets, ch) || strings.TrimSpace(msg.OpenID) != ""
	default:
		return HasUserChannel(targets, ch)
	}
}

func (s *NotificationService) dispatchNotification(notificationID uint64, msg notifier.NotificationMessage, targets map[string]notifierTarget) {
	for _, n := range s.notifiers {
		ch := n.Channel()
		if !shouldDispatchChannel(ch, n, targets, msg) {
			continue
		}

		go func(notif notifier.Notifier, channel string) {
			target := buildTarget(channel, targets, msg)
			delivery := model.NotificationDelivery{
				NotificationID: notificationID,
				Channel:        channel,
				Status:         model.DeliveryStatusPending,
				Target:         target.Masked,
			}
			s.db.Create(&delivery)

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			result, err := notif.Send(ctx, msg, target.NTarget)
			s.recordDelivery(&delivery, result, err)
		}(n, ch)
	}
}

type builtTarget struct {
	NTarget notifier.NotificationTarget
	Masked  string
}

func buildTarget(channel string, targets map[string]notifierTarget, msg notifier.NotificationMessage) builtTarget {
	t, ok := targets[channel]
	nt := notifier.NotificationTarget{}
	masked := ""
	switch channel {
	case model.ChannelWebSocket:
		masked = "websocket"
	case model.ChannelWeChatSubscribe:
		nt.OpenID = msg.OpenID
		if t.OpenID != "" {
			nt.OpenID = t.OpenID
		}
		masked = maskValue(nt.OpenID)
	case model.ChannelWecomWorkbench:
		nt.Secret = t.Secret
		nt.WecomUserid = t.Secret
		masked = maskValue(nt.Secret)
	case model.ChannelServerChan:
		nt.Secret = t.Secret
		masked = maskValue(nt.Secret)
	case model.ChannelBark:
		nt.Secret = t.Secret
		nt.Endpoint = t.Endpoint
		masked = maskValue(nt.Secret)
	case model.ChannelNtfy:
		nt.Topic = t.Topic
		nt.Endpoint = t.Endpoint
		nt.Secret = t.Secret
		masked = maskValue(nt.Topic)
	}
	if !ok && channel == model.ChannelWeChatSubscribe {
		nt.OpenID = msg.OpenID
		masked = maskValue(nt.OpenID)
	}
	return builtTarget{NTarget: nt, Masked: masked}
}

func maskValue(s string) string {
	if len(s) <= 6 {
		return "***"
	}
	return s[:3] + "***" + s[len(s)-3:]
}

func (s *NotificationService) recordDelivery(d *model.NotificationDelivery, result *notifier.SendResult, err error) {
	now := time.Now()
	updates := map[string]any{"updated_at": now}
	if result != nil {
		updates["status"] = result.Status
		updates["request_id"] = result.RequestID
		updates["error_code"] = result.ErrorCode
		updates["error_message"] = result.ErrorMessage
		if result.MaskedTarget != "" {
			updates["target"] = result.MaskedTarget
		}
		if result.Status == model.DeliveryStatusSent {
			updates["sent_at"] = now
		}
		if result.Status == model.DeliveryStatusFailed && result.Retryable {
			updates["status"] = model.DeliveryStatusPending
			updates["next_retry_at"] = s.nextRetryTime(d.RetryCount)
			updates["retry_count"] = d.RetryCount + 1
		}
	} else if err != nil {
		updates["status"] = model.DeliveryStatusSkipped
		updates["error_message"] = err.Error()
	}
	s.db.Model(d).Updates(updates)
}

func (s *NotificationService) nextRetryTime(retryCount int) *time.Time {
	intervals := config.AppConfig.Notification.Retry.IntervalsSec
	if retryCount >= len(intervals) {
		return nil
	}
	t := time.Now().Add(time.Duration(intervals[retryCount]) * time.Second)
	return &t
}

// DB 返回数据库实例。
func (s *NotificationService) DB() *gorm.DB {
	return s.db
}

// FlushUnreadWebSocket 用户上线后补推尚未通过 WebSocket 送达的未读通知。
func (s *NotificationService) FlushUnreadWebSocket(userID uint64) {
	if config.AppConfig == nil || !config.AppConfig.Notification.Enabled || !config.AppConfig.Notification.WebSocket.Enabled {
		return
	}
	if s.hub == nil || !s.hub.IsOnline(userID) {
		return
	}

	list, err := s.pendingWebSocketNotifications(userID)
	if err != nil || len(list) == 0 {
		return
	}

	for _, n := range list {
		if !s.hub.IsOnline(userID) {
			return
		}

		var order model.DailyOrder
		if err := s.db.First(&order, n.OrderID).Error; err != nil {
			log.Printf("[WebSocket] 补推跳过 notification=%d: 订单不存在", n.ID)
			continue
		}

		msg := notifier.NotificationMessage{
			NotificationID: n.ID,
			ReceiverUserID: n.ReceiverUserID,
			Title:          n.Title,
			Content:        n.Content,
			OrderID:        n.OrderID,
			Date:           dateutil.FormatYMD(order.Date),
			MealType:       order.MealType,
		}
		if s.hub.PushToUser(userID, notifier.BuildOrderCreatedPayload(msg)) {
			s.markWebSocketDeliverySent(n.ID)
		}
	}
}

func (s *NotificationService) pendingWebSocketNotifications(userID uint64) ([]model.Notification, error) {
	var list []model.Notification
	err := s.db.Where("receiver_user_id = ? AND status = ?", userID, model.NotificationStatusUnread).
		Where(`NOT EXISTS (
			SELECT 1 FROM notification_deliveries d
			WHERE d.notification_id = notifications.id
			AND d.channel = ?
			AND d.status = ?
		)`, model.ChannelWebSocket, model.DeliveryStatusSent).
		Order("created_at ASC").
		Limit(50).
		Find(&list).Error
	return list, err
}

func (s *NotificationService) markWebSocketDeliverySent(notificationID uint64) {
	now := time.Now()
	var d model.NotificationDelivery
	err := s.db.Where("notification_id = ? AND channel = ?", notificationID, model.ChannelWebSocket).
		First(&d).Error
	if err == gorm.ErrRecordNotFound {
		s.db.Create(&model.NotificationDelivery{
			NotificationID: notificationID,
			Channel:        model.ChannelWebSocket,
			Status:         model.DeliveryStatusSent,
			Target:         "online",
			SentAt:         &now,
		})
		return
	}
	if err != nil {
		return
	}
	s.db.Model(&d).Updates(map[string]any{
		"status":         model.DeliveryStatusSent,
		"target":         "online",
		"sent_at":        now,
		"error_message":  "",
		"error_code":     "",
		"next_retry_at":  nil,
		"updated_at":     now,
	})
}

// ListUnread 未读通知列表。
func (s *NotificationService) ListUnread(userID uint64) ([]model.Notification, error) {
	var list []model.Notification
	err := s.db.Where("receiver_user_id = ? AND status = ?", userID, model.NotificationStatusUnread).
		Order("created_at DESC").Limit(50).Find(&list).Error
	return list, err
}

// MarkRead 标记已读。
func (s *NotificationService) MarkRead(userID, notificationID uint64) error {
	now := time.Now()
	res := s.db.Model(&model.Notification{}).
		Where("id = ? AND receiver_user_id = ? AND status = ?", notificationID, userID, model.NotificationStatusUnread).
		Updates(map[string]any{"status": model.NotificationStatusRead, "read_at": now})
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return res.Error
}

// RetryPendingDeliveries 重试 pending 投递。
func (s *NotificationService) RetryPendingDeliveries() error {
	if config.AppConfig == nil || !config.AppConfig.Notification.Worker.Enabled {
		return nil
	}
	maxAttempts := config.AppConfig.Notification.Retry.MaxAttempts
	now := time.Now()

	var deliveries []model.NotificationDelivery
	s.db.Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", model.DeliveryStatusPending, now).
		Limit(100).Find(&deliveries)

	for _, d := range deliveries {
		d := d
		if d.RetryCount >= maxAttempts {
			s.db.Model(&d).Updates(map[string]any{
				"status": model.DeliveryStatusFailed, "error_message": "超过最大重试次数",
			})
			continue
		}
		go func() {
			var n model.Notification
			if err := s.db.First(&n, d.NotificationID).Error; err != nil {
				return
			}
			var order model.DailyOrder
			if err := s.db.Preload("Recipe").Preload("Adder").First(&order, n.OrderID).Error; err != nil {
				return
			}
			var user model.User
			s.db.First(&user, n.ReceiverUserID)

			msg := notifier.NotificationMessage{
				NotificationID: n.ID,
				ReceiverUserID: n.ReceiverUserID,
				Title:          n.Title,
				Content:        n.Content,
				OrderID:        n.OrderID,
				OpenID:         user.OpenID,
			}
			if order.Recipe != nil {
				msg.RecipeName = order.Recipe.Name
				msg.Ingredients = order.Recipe.Ingredients
				msg.RecipeCoverURL = order.Recipe.CoverURL
			}
			if order.Adder != nil {
				msg.AdderName = order.Adder.Nickname
			}
			msg.MealType = order.MealType
			msg.Date = order.Date
			msg.Note = order.Note

			targets := NewNotificationChannelService(s.db).GetEnabledTargets(n.ReceiverUserID, &user)

			var notif notifier.Notifier
			for _, nn := range s.notifiers {
				if nn.Channel() == d.Channel {
					notif = nn
					break
				}
			}
			if notif == nil || !shouldDispatchChannel(d.Channel, notif, targets, msg) {
				s.db.Model(&d).Updates(map[string]any{
					"status": model.DeliveryStatusSkipped, "error_message": "通道未配置或已禁用",
				})
				return
			}

			bt := buildTarget(d.Channel, targets, msg)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			result, err := notif.Send(ctx, msg, bt.NTarget)
			s.recordDelivery(&d, result, err)
		}()
	}
	return nil
}

// StartRetryWorker 启动重试 worker。
func (s *NotificationService) StartRetryWorker() {
	interval := time.Duration(config.AppConfig.Notification.Worker.PollIntervalSec) * time.Second
	go func() {
		ticker := time.NewTicker(interval)
		for range ticker.C {
			if err := s.RetryPendingDeliveries(); err != nil {
				log.Printf("[通知Worker] 重试失败: %v", err)
			}
		}
	}()
}
