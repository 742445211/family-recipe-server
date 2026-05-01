package model

import (
	"time"

	"gorm.io/gorm"
)

// DailyOrder 每日点菜（每条记录 = 一道菜在某个餐次被点了一次）
type DailyOrder struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID  uint64         `gorm:"not null;uniqueIndex:uk_family_date_meal_recipe;index:idx_family_date_meal" json:"family_id"`
	Date      string         `gorm:"type:date;not null;uniqueIndex:uk_family_date_meal_recipe;index:idx_family_date_meal" json:"date"`
	MealType  string         `gorm:"size:20;not null;default:dinner;uniqueIndex:uk_family_date_meal_recipe;index:idx_family_date_meal" json:"meal_type"` // breakfast/lunch/dinner
	RecipeID  uint64         `gorm:"not null;uniqueIndex:uk_family_date_meal_recipe;index" json:"recipe_id"`
	AddedBy   uint64         `gorm:"not null" json:"added_by"`
	Quantity  int            `gorm:"default:1" json:"quantity"`
	Note      string         `gorm:"size:200" json:"note"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Recipe *Recipe `gorm:"foreignKey:RecipeID" json:"recipe,omitempty"`
	Adder  *User   `gorm:"foreignKey:AddedBy" json:"adder,omitempty"`
}

func (DailyOrder) TableName() string { return "daily_orders" }
