// 全局菜谱库数据模型：catalog_recipes 表，同一 name_key 可有多条 variant（不同做法）。
package model

import "time"

// CatalogRecipe 全局菜谱库（跨家庭共享，含同一菜名的多种做法）。
type CatalogRecipe struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string    `gorm:"size:200;not null" json:"name"`
	NameKey       string    `gorm:"size:200;not null;index:idx_catalog_name_key;uniqueIndex:uk_catalog_name_hash,priority:1" json:"name_key"`
	VariantLabel  string    `gorm:"size:100;not null;default:经典做法" json:"variant_label"`
	IsDefault     bool      `gorm:"not null;default:false" json:"is_default"`
	Category      string    `gorm:"size:50;default:其他" json:"category"`
	Ingredients   string    `gorm:"type:json" json:"ingredients"`
	Seasonings    string    `gorm:"type:json" json:"seasonings"`
	Steps         string    `gorm:"type:json" json:"steps"`
	CookTime      int       `gorm:"default:0" json:"cook_time"`
	Difficulty    string    `gorm:"size:20;default:medium" json:"difficulty"`
	CoverURL      string    `gorm:"size:500" json:"cover_url"`
	Tips          string    `gorm:"type:text" json:"tips"`
	Source        string    `gorm:"size:32;not null" json:"source"` // ai_search | ai_recommend
	ContentHash   string    `gorm:"size:64;not null;index:idx_catalog_content_hash;uniqueIndex:uk_catalog_name_hash,priority:2" json:"content_hash"`
	UseCount      int       `gorm:"not null;default:0" json:"use_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName 指定表名为 catalog_recipes。
func (CatalogRecipe) TableName() string { return "catalog_recipes" }
