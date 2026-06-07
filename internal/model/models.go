// Package model 定义数据库模型（GORM 实体）。
// 包含家庭、用户、家庭成员、菜谱、菜单、收藏、每日点菜等核心业务表。
// 所有模型均使用软删除（gorm.DeletedAt）。
package model

import (
	"time"

	"gorm.io/gorm"
)

// Family 家庭表，一个家庭包含多个成员和菜谱。
// 通过 InviteCode（唯一邀请码）支持成员加入。
type Family struct {
	ID         uint64         `gorm:"primaryKey;autoIncrement" json:"id"`          // 主键
	Name       string         `gorm:"size:100;not null" json:"name"`              // 家庭名称
	InviteCode string         `gorm:"size:8;uniqueIndex;not null" json:"invite_code"` // 6位邀请码
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"` // 软删除
}

// TableName 指定表名为 families。
func (Family) TableName() string { return "families" }

// User 用户表，通过微信 OpenID 唯一标识。
// CurrentFamilyID 记录用户当前选中的家庭。
type User struct {
	ID              uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	OpenID          string         `gorm:"size:64;uniqueIndex;not null;column:openid" json:"openid"`  // 微信 OpenID
	UnionID         string         `gorm:"size:64;column:unionid" json:"unionid"`                     // 微信 UnionID
	Nickname        string         `gorm:"size:100" json:"nickname"`                                  // 昵称
	AvatarURL       string         `gorm:"size:500" json:"avatar_url"`                                // 头像 URL
	WecomUserid     string         `gorm:"size:64;column:wecom_userid" json:"wecom_userid,omitempty"` // 企业微信成员 UserID
	CurrentFamilyID *uint64        `json:"current_family_id"`                                         // 当前选中家庭（指针允许 NULL）
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名为 users。
func (User) TableName() string { return "users" }

// FamilyMember 家庭成员关联表，记录用户在家庭中的角色和厨师身份。
// 通过 (family_id, user_id) 联合唯一索引防止重复加入。
type FamilyMember struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID  uint64         `gorm:"not null;uniqueIndex:uk_family_user" json:"family_id"`  // 所属家庭
	UserID    uint64         `gorm:"not null;uniqueIndex:uk_family_user" json:"user_id"`    // 用户 ID
	Role      string         `gorm:"size:20;default:member" json:"role"`                    // 角色：owner / member
	IsChef    bool           `gorm:"default:false" json:"is_chef"`                          // 是否厨师（接收点菜通知）
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Family *Family `gorm:"foreignKey:FamilyID" json:"family,omitempty"` // 关联家庭
	User   *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`     // 关联用户
}

// TableName 指定表名为 family_members。
func (FamilyMember) TableName() string { return "family_members" }

// Recipe 菜谱表，记录每道菜的详细信息。
// 食材（Ingredients）、调料（Seasonings）、步骤（Steps）以 JSON 数组存储。
type Recipe struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"size:200;not null" json:"name"`        // 菜名
	Category    string         `gorm:"size:50;default:其他" json:"category"`    // 分类（如：家常菜、川菜等）
	Ingredients string         `gorm:"type:json" json:"ingredients"`         // 食材列表（JSON 数组）
	Seasonings  string         `gorm:"type:json" json:"seasonings"`          // 调料列表（JSON 数组）
	Steps       string         `gorm:"type:json" json:"steps"`               // 步骤列表（JSON 数组）
	CookTime    int            `gorm:"default:0" json:"cook_time"`           // 烹饪耗时（分钟）
	Difficulty  string         `gorm:"size:20;default:medium" json:"difficulty"` // 难度：easy/medium/hard
	ImageKey    string         `gorm:"size:200" json:"image_key"`            // OSS 存储 key
	CoverURL    string         `gorm:"size:500" json:"cover_url"`            // 封面图完整 URL
	Tips        string         `gorm:"type:text" json:"tips"`                // 烹饪小贴士
	CreatorID   uint64         `gorm:"not null" json:"creator_id"`           // 创建者
	FamilyID    uint64         `gorm:"not null;index" json:"family_id"`      // 所属家庭
	IsPublic    bool           `gorm:"default:true" json:"is_public"`        // 是否公开
	CookCount   int            `gorm:"default:0" json:"cook_count"`          // 累计烹饪次数
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Creator *User   `gorm:"foreignKey:CreatorID" json:"creator,omitempty"` // 创建者信息
	Family  *Family `gorm:"foreignKey:FamilyID" json:"family,omitempty"`   // 所属家庭信息
}

// TableName 指定表名为 recipes。
func (Recipe) TableName() string { return "recipes" }

// Menu 点菜单（已废弃，保留兼容）。
// 新业务使用 DailyOrder 按日期点菜，此结构保留以满足旧数据兼容。
type Menu struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID  uint64         `gorm:"not null;index:idx_family_date" json:"family_id"`     // 所属家庭
	Name      string         `gorm:"size:100;not null" json:"name"`                       // 菜单名称
	Date      string         `gorm:"type:date;not null;index:idx_family_date" json:"date"` // 日期
	Status    string         `gorm:"size:20;default:draft" json:"status"`                 // 状态：draft/confirmed
	CreatorID uint64         `gorm:"not null" json:"creator_id"`                          // 创建者
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Items   []MenuWithItem `gorm:"foreignKey:MenuID" json:"items,omitempty"`  // 菜单项
	Creator *User          `gorm:"foreignKey:CreatorID" json:"creator,omitempty"` // 创建者信息
}

// TableName 指定表名为 menus。
func (Menu) TableName() string { return "menus" }

// MenuItem 点菜明细（已废弃）。
type MenuItem struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	MenuID    uint64         `gorm:"not null;uniqueIndex:uk_menu_recipe" json:"menu_id"`  // 所属菜单
	RecipeID  uint64         `gorm:"not null;uniqueIndex:uk_menu_recipe" json:"recipe_id"` // 菜谱 ID（同一菜单不可重复点同一道菜）
	AddedBy   uint64         `gorm:"not null" json:"added_by"`                             // 点菜人
	Quantity  int            `gorm:"default:1" json:"quantity"`                            // 数量
	Note      string         `gorm:"size:200" json:"note"`                                 // 备注
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Recipe *Recipe `gorm:"foreignKey:RecipeID" json:"recipe,omitempty"` // 关联菜谱
	Adder  *User   `gorm:"foreignKey:AddedBy" json:"adder,omitempty"`   // 点菜人信息
}

// TableName 指定表名为 menu_items。
func (MenuItem) TableName() string { return "menu_items" }

// MenuWithItem 点菜单 + 菜谱信息（用于列表展示的扁平化结构）。
// 内嵌 MenuItem 并附加菜谱名称、分类、封面等冗余字段。
type MenuWithItem struct {
	MenuItem
	RecipeName     string `json:"recipe_name"`     // 菜谱名称
	RecipeCategory string `json:"recipe_category"` // 菜谱分类
	RecipeCoverURL string `json:"recipe_cover_url"` // 菜谱封面 URL
}

// Favorite 收藏表，记录用户收藏的菜谱。
// 通过 (user_id, recipe_id) 联合唯一索引防止重复收藏。
type Favorite struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64         `gorm:"not null;uniqueIndex:uk_user_recipe" json:"user_id"`   // 用户 ID
	RecipeID  uint64         `gorm:"not null;uniqueIndex:uk_user_recipe" json:"recipe_id"` // 菜谱 ID
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Recipe *Recipe `gorm:"foreignKey:RecipeID" json:"recipe,omitempty"` // 关联菜谱
}

// TableName 指定表名为 favorites。
func (Favorite) TableName() string { return "favorites" }
