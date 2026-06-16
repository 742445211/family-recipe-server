package model

import (
	"time"

	"gorm.io/gorm"
)

// FridgeItem 家庭冰箱库存食材。
type FridgeItem struct {
	ID         uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID   uint64         `gorm:"not null;index" json:"family_id"`
	Name       string         `gorm:"size:100;not null" json:"name"`
	Amount     string         `gorm:"size:50" json:"amount"`
	ExpiryDate *string        `gorm:"type:date" json:"expiry_date,omitempty"`
	Note       string         `gorm:"size:200" json:"note"`
	Source     string         `gorm:"size:20;not null" json:"source"` // manual | photo
	ScanID     *uint64        `json:"scan_id,omitempty"`
	AddedBy    uint64         `gorm:"not null" json:"added_by"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (FridgeItem) TableName() string { return "fridge_items" }

// FridgeScan 冰箱拍照识别任务。
type FridgeScan struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID         uint64     `gorm:"not null;index" json:"family_id"`
	UserID           uint64     `gorm:"not null" json:"user_id"`
	TaskID           string     `gorm:"size:36;uniqueIndex" json:"task_id"`
	ImageKey         string     `gorm:"size:200" json:"image_key"`
	ImageURL         string     `gorm:"size:500" json:"image_url"`
	Status           string     `gorm:"size:20;not null" json:"status"`
	RecognizedItems  string     `gorm:"type:json" json:"recognized_items,omitempty"`
	ErrorMsg         string     `gorm:"type:text" json:"error_msg,omitempty"`
	ConfirmedAt      *time.Time `json:"confirmed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (FridgeScan) TableName() string { return "fridge_scans" }
