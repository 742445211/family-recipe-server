// Package notifier - WebSocket 在线推送通道。
package notifier

import (
	"context"

	"recipe-server/pkg/dateutil"
)

// WebSocketNotifier 通过 WebSocket Hub 在线推送。
type WebSocketNotifier struct {
	enabled bool
	hub     WebSocketHub
}

// WebSocketHub WebSocket 推送接口。
type WebSocketHub interface {
	PushToUser(userID uint64, payload map[string]any) bool
	IsOnline(userID uint64) bool
}

// NewWebSocketNotifier 创建 WebSocket 通知器。
func NewWebSocketNotifier(enabled bool, hub WebSocketHub) *WebSocketNotifier {
	return &WebSocketNotifier{enabled: enabled, hub: hub}
}

func (n *WebSocketNotifier) Channel() string { return "websocket" }
func (n *WebSocketNotifier) Enabled() bool   { return n.enabled && n.hub != nil }

func (n *WebSocketNotifier) Send(ctx context.Context, msg NotificationMessage, target NotificationTarget) (*SendResult, error) {
	_ = ctx
	_ = target
	if !n.Enabled() {
		return SkippedResult("websocket disabled", ""), nil
	}
	payload := BuildOrderCreatedPayload(msg)
	if n.hub.PushToUser(msg.ReceiverUserID, payload) {
		return &SendResult{Status: "sent", MaskedTarget: "online"}, nil
	}
	return SkippedResult("user offline", "offline"), nil
}

// BuildOrderCreatedPayload 构造点菜 WebSocket 推送 JSON 体。
func BuildOrderCreatedPayload(msg NotificationMessage) map[string]any {
	payload := map[string]any{
		"type":        "ORDER_CREATED",
		"title":       msg.Title,
		"content":     msg.Content,
		"order_id":    msg.OrderID,
		"date":        dateutil.FormatYMD(msg.Date),
		"meal_type":   msg.MealType,
		"recipe_name": msg.RecipeName,
		"adder_name":  msg.AdderName,
		"ingredients": FormatIngredients(msg.Ingredients),
	}
	if msg.NotificationID > 0 {
		payload["notification_id"] = msg.NotificationID
	}
	return payload
}
