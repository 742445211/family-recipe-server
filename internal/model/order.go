package model

import "time"

// DailyOrder 每日点菜（每条记录 = 一道菜被点了一次）
type DailyOrder struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID  uint64    `gorm:"not null;index:idx_family_date" json:"family_id"`
	Date      string    `gorm:"type:date;not null;index:idx_family_date" json:"date"`
	RecipeID  uint64    `gorm:"not null" json:"recipe_id"`
	AddedBy   uint64    `gorm:"not null" json:"added_by"`
	Quantity  int       `gorm:"default:1" json:"quantity"`
	Note      string    `gorm:"size:200" json:"note"`
	CreatedAt time.Time `json:"created_at"`

	Recipe *Recipe `gorm:"foreignKey:RecipeID" json:"recipe,omitempty"`
	Adder  *User   `gorm:"foreignKey:AddedBy" json:"adder,omitempty"`
}

func (DailyOrder) TableName() string { return "daily_orders" }
