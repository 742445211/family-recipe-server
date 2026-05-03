// Package model - 订单模型定义（每日点菜）。
//
// 本文件定义 DailyOrder（每日点菜）模型，是 model 包中订单相关的核心实体。
// DailyOrder 替代了旧的 Menu/MenuItem 模型，支持按日期 + 餐次维度点菜，
// 更适合日常家庭场景。
package model

import (
	"time"

	"gorm.io/gorm"
)

// DailyOrder 每日点菜表，每条记录表示一道菜在某个日期某个餐次被点了一次。
//
// 业务规则：
//   - 支持 breakfast（早餐）、lunch（午餐）、dinner（晚餐）三个餐次
//   - 同一家庭同日期同餐次不允许重复点同一道菜（由 handler 层保证）
//   - 每天每家可为每个餐次独立点菜，互不干扰
//
// 索引设计：
//   - idx_family_date_meal 联合索引覆盖 FamilyID + Date + MealType，加速按家庭日期餐次查询
//   - RecipeID 单列索引，加速按菜谱反查订单
type DailyOrder struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID  uint64         `gorm:"not null;index:idx_family_date_meal" json:"family_id"`   // 家庭 ID
	Date      string         `gorm:"type:date;not null;index:idx_family_date_meal" json:"date"` // 日期（YYYY-MM-DD）
	MealType  string         `gorm:"size:20;not null;default:dinner;index:idx_family_date_meal" json:"meal_type"` // 餐次：breakfast/lunch/dinner
	RecipeID  uint64         `gorm:"not null;index" json:"recipe_id"`                        // 菜谱 ID
	AddedBy   uint64         `gorm:"not null" json:"added_by"`                               // 点菜人（用户 ID）
	Quantity  int            `gorm:"default:1" json:"quantity"`                              // 数量（默认 1）
	Note      string         `gorm:"size:200" json:"note"`                                   // 备注（如"少放盐"）
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // 软删除（GORM 自动管理）

	// 关联查询
	Recipe *Recipe `gorm:"foreignKey:RecipeID" json:"recipe,omitempty"` // 关联菜谱详情
	Adder  *User   `gorm:"foreignKey:AddedBy" json:"adder,omitempty"`   // 点菜人用户信息
}

// TableName 指定 DailyOrder 对应的数据库表名。
// GORM 默认会以结构体名复数形式命名（daily_orders），此处显式声明以保持一致性。
func (DailyOrder) TableName() string { return "daily_orders" }
