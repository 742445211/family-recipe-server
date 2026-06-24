// 通知相关数据模型：notifications、notification_deliveries、notification_channels 表及通道常量。
package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	NotificationTypeOrderCreated = "ORDER_CREATED"
	NotificationStatusUnread     = "unread"
	NotificationStatusRead       = "read"

	ChannelWebSocket       = "websocket"
	ChannelWeChatSubscribe   = "wechat_subscribe"
	ChannelWecomWorkbench    = "wecom_workbench"
	ChannelServerChan        = "server_chan"
	ChannelBark              = "bark"
	ChannelNtfy              = "ntfy"

	DeliveryStatusPending = "pending"
	DeliveryStatusSent    = "sent"
	DeliveryStatusFailed  = "failed"
	DeliveryStatusSkipped = "skipped"
)

// Notification 厨师点菜通知记录。
type Notification struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID       uint64         `gorm:"not null;index:idx_family_created" json:"family_id"`
	ReceiverUserID uint64         `gorm:"not null;index:idx_receiver_status" json:"receiver_user_id"`
	OrderID        uint64         `gorm:"not null;index:idx_order_receiver" json:"order_id"`
	Type           string         `gorm:"size:50;not null" json:"type"`
	Title          string         `gorm:"size:100;not null" json:"title"`
	Content        string         `gorm:"size:500;not null" json:"content"`
	Status         string         `gorm:"size:20;not null;default:unread;index:idx_receiver_status" json:"status"`
	CreatedAt      time.Time      `json:"created_at"`
	ReadAt         *time.Time     `json:"read_at,omitempty"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Notification) TableName() string { return "notifications" }

// NotificationDelivery 通知各通道发送结果。
type NotificationDelivery struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	NotificationID uint64     `gorm:"not null;index:idx_notification_channel" json:"notification_id"`
	Channel        string     `gorm:"size:30;not null;index:idx_notification_channel" json:"channel"`
	Status         string     `gorm:"size:20;not null;default:pending;index:idx_status_retry" json:"status"`
	Target         string     `gorm:"size:255" json:"target"`
	RequestID      string     `gorm:"size:100" json:"request_id"`
	ErrorCode      string     `gorm:"size:50" json:"error_code"`
	ErrorMessage   string     `gorm:"size:500" json:"error_message"`
	RetryCount     int        `gorm:"not null;default:0" json:"retry_count"`
	NextRetryAt    *time.Time `gorm:"index:idx_status_retry" json:"next_retry_at,omitempty"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (NotificationDelivery) TableName() string { return "notification_deliveries" }

// NotificationChannel 厨师配置的外部通知通道。
type NotificationChannel struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64         `gorm:"not null;index:idx_user_channel" json:"user_id"`
	Channel   string         `gorm:"size:30;not null;index:idx_user_channel" json:"channel"`
	Enabled   bool           `gorm:"not null;default:true" json:"enabled"`
	Endpoint  string         `gorm:"size:500" json:"endpoint,omitempty"`
	Secret    string         `gorm:"size:500" json:"-"`
	Topic     string         `gorm:"size:200" json:"topic,omitempty"`
	Extra     string         `gorm:"type:json" json:"extra,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (NotificationChannel) TableName() string { return "notification_channels" }
