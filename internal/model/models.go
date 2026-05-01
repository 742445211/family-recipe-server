package model

import (
	"time"

	"gorm.io/gorm"
)

// Family 家庭
type Family struct {
	ID         uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string         `gorm:"size:100;not null" json:"name"`
	InviteCode string         `gorm:"size:8;uniqueIndex;not null" json:"invite_code"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Family) TableName() string { return "families" }

// User 用户
type User struct {
	ID              uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	OpenID          string         `gorm:"size:64;uniqueIndex;not null;column:openid" json:"openid"`
	UnionID         string         `gorm:"size:64;column:unionid" json:"unionid"`
	Nickname        string         `gorm:"size:100" json:"nickname"`
	AvatarURL       string         `gorm:"size:500" json:"avatar_url"`
	CurrentFamilyID *uint64        `json:"current_family_id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string { return "users" }

// FamilyMember 家庭成员
type FamilyMember struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID  uint64         `gorm:"not null;uniqueIndex:uk_family_user" json:"family_id"`
	UserID    uint64         `gorm:"not null;uniqueIndex:uk_family_user" json:"user_id"`
	Role      string         `gorm:"size:20;default:member" json:"role"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Family *Family `gorm:"foreignKey:FamilyID" json:"family,omitempty"`
	User   *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (FamilyMember) TableName() string { return "family_members" }

// Recipe 菜谱
type Recipe struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"size:200;not null" json:"name"`
	Category    string         `gorm:"size:50;default:其他" json:"category"`
	Ingredients string         `gorm:"type:json" json:"ingredients"`
	Seasonings  string         `gorm:"type:json" json:"seasonings"`
	Steps       string         `gorm:"type:json" json:"steps"`
	CookTime    int            `gorm:"default:0" json:"cook_time"`
	Difficulty  string         `gorm:"size:20;default:medium" json:"difficulty"`
	ImageKey    string         `gorm:"size:200" json:"image_key"`
	CoverURL    string         `gorm:"size:500" json:"cover_url"`
	Tips        string         `gorm:"type:text" json:"tips"`
	CreatorID   uint64         `gorm:"not null" json:"creator_id"`
	FamilyID    uint64         `gorm:"not null;index" json:"family_id"`
	IsPublic    bool           `gorm:"default:true" json:"is_public"`
	CookCount   int            `gorm:"default:0" json:"cook_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Creator *User   `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
	Family  *Family `gorm:"foreignKey:FamilyID" json:"family,omitempty"`
}

func (Recipe) TableName() string { return "recipes" }

// Menu 点菜单（已废弃，保留兼容）
type Menu struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	FamilyID  uint64         `gorm:"not null;index:idx_family_date" json:"family_id"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	Date      string         `gorm:"type:date;not null;index:idx_family_date" json:"date"`
	Status    string         `gorm:"size:20;default:draft" json:"status"`
	CreatorID uint64         `gorm:"not null" json:"creator_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Items   []MenuWithItem `gorm:"foreignKey:MenuID" json:"items,omitempty"`
	Creator *User          `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
}

func (Menu) TableName() string { return "menus" }

// MenuItem 点菜明细（已废弃）
type MenuItem struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	MenuID    uint64         `gorm:"not null;uniqueIndex:uk_menu_recipe" json:"menu_id"`
	RecipeID  uint64         `gorm:"not null;uniqueIndex:uk_menu_recipe" json:"recipe_id"`
	AddedBy   uint64         `gorm:"not null" json:"added_by"`
	Quantity  int            `gorm:"default:1" json:"quantity"`
	Note      string         `gorm:"size:200" json:"note"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Recipe *Recipe `gorm:"foreignKey:RecipeID" json:"recipe,omitempty"`
	Adder  *User   `gorm:"foreignKey:AddedBy" json:"adder,omitempty"`
}

func (MenuItem) TableName() string { return "menu_items" }

// MenuWithItem 点菜单+菜谱信息
type MenuWithItem struct {
	MenuItem
	RecipeName     string `json:"recipe_name"`
	RecipeCategory string `json:"recipe_category"`
	RecipeCoverURL string `json:"recipe_cover_url"`
}

// Favorite 收藏
type Favorite struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64         `gorm:"not null;uniqueIndex:uk_user_recipe" json:"user_id"`
	RecipeID  uint64         `gorm:"not null;uniqueIndex:uk_user_recipe" json:"recipe_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Recipe *Recipe `gorm:"foreignKey:RecipeID" json:"recipe,omitempty"`
}

func (Favorite) TableName() string { return "favorites" }
