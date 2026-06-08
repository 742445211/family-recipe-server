package notifier

import (
	"context"
	"errors"
	"strings"

	"recipe-server/pkg/dateutil"
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
	Note            string
	Ingredients     string // 菜谱食材 JSON（[{"name":"","amount":""}]）
	RecipeCoverURL  string // 菜谱封面图 URL，用于企微图文卡片顶部 picurl
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
	b.WriteString(dateutil.FormatYMD(msg.Date))
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

// BuildOrderNewsDescription 构建企业微信 news 图文卡片的纯文本描述（news 不支持 HTML）。
func BuildOrderNewsDescription(msg NotificationMessage) string {
	meal := MealName(msg.MealType)
	var b strings.Builder
	b.WriteString(dateutil.FormatYMD(msg.Date))
	if meal != "" {
		b.WriteString(" ")
		b.WriteString(meal)
	}
	b.WriteString("\n")
	b.WriteString(msg.RecipeName)
	b.WriteString(" 1份")
	if strings.TrimSpace(msg.AdderName) != "" {
		b.WriteString("\n点菜人：")
		b.WriteString(msg.AdderName)
	}
	if ing := FormatIngredients(msg.Ingredients); ing != "" {
		b.WriteString("\n食材：")
		b.WriteString(truncateRunes(ing, 80))
	}
	if strings.TrimSpace(msg.Note) != "" {
		b.WriteString("\n备注：")
		b.WriteString(msg.Note)
	}
	return b.String()
}

// BuildOrderCardDescription 构建企业微信 textcard 卡片描述（支持有限 HTML：gray/highlight/normal）。
func BuildOrderCardDescription(msg NotificationMessage) string {
	meal := MealName(msg.MealType)
	var b strings.Builder
	b.WriteString(`<div class="gray">`)
	b.WriteString(dateutil.FormatYMD(msg.Date))
	if meal != "" {
		b.WriteString(" ")
		b.WriteString(meal)
	}
	b.WriteString("</div>")
	b.WriteString(`<div class="highlight">`)
	b.WriteString(msg.RecipeName)
	b.WriteString(" 1份</div>")
	if strings.TrimSpace(msg.AdderName) != "" {
		b.WriteString(`<div class="normal">点菜人：`)
		b.WriteString(msg.AdderName)
		b.WriteString("</div>")
	}
	if ing := FormatIngredients(msg.Ingredients); ing != "" {
		b.WriteString(`<div class="normal">食材：`)
		b.WriteString(truncateRunes(ing, 60))
		b.WriteString("</div>")
	}
	if strings.TrimSpace(msg.Note) != "" {
		b.WriteString(`<div class="normal">备注：`)
		b.WriteString(msg.Note)
		b.WriteString("</div>")
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
