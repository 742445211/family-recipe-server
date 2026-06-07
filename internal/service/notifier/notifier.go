package notifier

import (
	"context"
	"errors"
	"strings"
)

// NotificationMessage 通知消息体。
type NotificationMessage struct {
	NotificationID uint64
	ReceiverUserID uint64
	Title          string
	Content        string
	OrderID        uint64
	RecipeName     string
	AdderName      string
	MealType       string
	Date           string
	Page           string
	OpenID         string
	Note           string
	Ingredients    string // 菜谱食材 JSON（[{"name":"","amount":""}]）
}

// NotificationTarget 用户级通道目标。
type NotificationTarget struct {
	OpenID      string
	WecomUserid string
	Secret      string
	Endpoint    string
	Topic       string
}

// SendResult 发送结果。
type SendResult struct {
	Status       string
	RequestID    string
	ErrorCode    string
	ErrorMessage string
	Retryable    bool
	MaskedTarget string
}

// Notifier 通知通道接口。
type Notifier interface {
	Channel() string
	Enabled() bool
	Send(ctx context.Context, msg NotificationMessage, target NotificationTarget) (*SendResult, error)
}

// MealName 餐次英文转中文。
func MealName(mealType string) string {
	m := map[string]string{"breakfast": "早餐", "lunch": "午餐", "dinner": "晚餐"}
	if name, ok := m[mealType]; ok {
		return name
	}
	return mealType
}

// BuildOrderContent 构建点菜通知正文（WebSocket / 企微 / Server酱 / Bark / ntfy 等，不含订阅模板）。
func BuildOrderContent(msg NotificationMessage) string {
	meal := MealName(msg.MealType)
	var b strings.Builder
	b.WriteString("日期：")
	b.WriteString(msg.Date)
	b.WriteString(" ")
	b.WriteString(meal)
	b.WriteString("\n")
	b.WriteString(msg.RecipeName)
	b.WriteString(" 1份，点菜人：")
	b.WriteString(msg.AdderName)
	if ing := FormatIngredients(msg.Ingredients); ing != "" {
		b.WriteString("\n食材：")
		b.WriteString(ing)
	}
	if strings.TrimSpace(msg.Note) != "" {
		b.WriteString("\n备注：")
		b.WriteString(msg.Note)
	}
	return b.String()
}

// ErrSkipped 表示通道跳过（如用户离线）。
var ErrSkipped = errors.New("notification skipped")

// SkippedResult 返回 skipped 结果。
func SkippedResult(reason, masked string) *SendResult {
	return &SendResult{
		Status:       "skipped",
		ErrorMessage: reason,
		MaskedTarget: masked,
	}
}
