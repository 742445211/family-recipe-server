// Package service - 菜谱管理服务。
//
// 本文件实现菜谱（Recipe）的完整 CRUD 操作，是食谱管理后台的核心服务。
// 支持按家庭隔离、关键词搜索、分类筛选、分页查询，以及烹饪次数统计。
//
// 核心功能：
//   - 菜谱的创建、更新、删除
//   - 菜谱详情查询（含创建者信息预加载）
//   - 菜谱列表查询（多条件过滤 + 分页）
//   - 烹饪次数累加（每次做菜后调用）
//
// 数据模型：model.Recipe → recipes 表（使用 GORM 软删除）
// 关联预加载：Creator → users 表（显示菜谱创建者昵称/头像）
package service

import (
	"errors"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

// ErrRecipeNotInFamily 菜谱不存在或不属于当前家庭。
var ErrRecipeNotInFamily = errors.New("菜谱不存在或不属于当前家庭")

// RecipeService 菜谱管理服务，封装 Recipe 的数据库操作。
// 通过 GORM 操作 recipes 表，支持多条件查询和关联预加载。
type RecipeService struct {
	db *gorm.DB // GORM 数据库实例
}

// NewRecipeService 创建菜谱服务实例。
//
// 参数:
//   - db *gorm.DB - GORM 数据库连接
//
// 返回值:
//   - *RecipeService - 菜谱服务指针
func NewRecipeService(db *gorm.DB) *RecipeService {
	return &RecipeService{db: db}
}

// Create 创建一条新菜谱记录。
//
// 参数:
//   - r *model.Recipe - 待创建的菜谱对象（需包含 Name、FamilyID、CreatorID 等必填字段）
//
// 返回值:
//   - error - GORM INSERT 失败时返回错误
//
// GORM 操作:
//
//	db.Create(r) → INSERT INTO recipes (...) VALUES (...)
func (s *RecipeService) Create(r *model.Recipe) error {
	wantPublic := r.IsPublic
	if err := s.db.Omit("IsPublic").Create(r).Error; err != nil {
		return err
	}
	// 显式落库 is_public（含 false），避免 GORM/SQLite 对 bool 零值的默认行为。
	if err := s.db.Model(r).UpdateColumn("is_public", wantPublic).Error; err != nil {
		return err
	}
	r.IsPublic = wantPublic
	return nil
}

// Update 更新菜谱信息（按主键 ID 匹配，只更新非零值字段）。
//
// 参数:
//   - r *model.Recipe - 需更新的菜谱对象（ID 必填，其他字段按需设置）
//
// 返回值:
//   - error - 更新失败时返回错误
//
// GORM 操作:
//
//	db.Model(&Recipe{}).Where("id = ?", r.ID).Updates(r)
//	→ UPDATE recipes SET ... WHERE id = ? AND deleted_at IS NULL
//
// 说明:
//   - Updates 使用 struct 时只更新非零值字段，零值字段不会写入数据库
//   - 如需将某字段重置为零值，应使用 Update + map 或 UpdateColumn
func (s *RecipeService) Update(r *model.Recipe, familyID uint64) error {
	res := s.db.Model(&model.Recipe{}).Where("id = ? AND family_id = ?", r.ID, familyID).
		Omit("FamilyID", "CreatorID", "ID").
		Updates(r)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRecipeNotInFamily
	}
	return nil
}

// Delete 删除菜谱（软删除），仅允许创建者本人操作。
//
// 参数:
//   - id     uint64 - 菜谱 ID
//   - userID uint64 - 操作人用户 ID
//
// 返回值:
//   - error - 删除失败时返回错误（如记录不存在或不属于该用户）
//
// GORM 操作:
//
//	db.Where("id = ? AND creator_id = ?", id, userID).Delete(&model.Recipe{})
//	→ UPDATE recipes SET deleted_at = NOW() WHERE id = ? AND creator_id = ? AND deleted_at IS NULL
//
// 说明:
//   - GORM 对软删除模型执行 Delete 时 UPDATE deleted_at 而非物理删除
//   - creator_id = userID 条件确保只有创建者可以删除
func (s *RecipeService) Delete(id, userID, familyID uint64) error {
	res := s.db.Where("id = ? AND creator_id = ? AND family_id = ?", id, userID, familyID).Delete(&model.Recipe{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRecipeNotInFamily
	}
	return nil
}

// GetByID 根据 ID 获取菜谱详情，同时预加载创建者信息。
//
// 参数:
//   - id uint64 - 菜谱主键 ID
//
// 返回值:
//   - *model.Recipe - 菜谱对象（含 Creator 预加载）
//   - error         - 记录不存在时返回 gorm.ErrRecordNotFound
//
// GORM 操作:
//
//	db.Preload("Creator").First(&r, id)
//	→ SELECT * FROM recipes WHERE id = ? AND deleted_at IS NULL LIMIT 1
//	→ SELECT * FROM users WHERE id = ? (Preload Creator)
// applyRecipeReadScope 读取可见范围：本家庭全部菜谱，或 is_public=1 的公开菜谱。
func applyRecipeReadScope(q *gorm.DB, familyID uint64) *gorm.DB {
	if familyID > 0 {
		return q.Where("family_id = ? OR is_public = ?", familyID, true)
	}
	return q.Where("is_public = ?", true)
}

func (s *RecipeService) GetByID(id, familyID uint64) (*model.Recipe, error) {
	var r model.Recipe
	q := applyRecipeReadScope(s.db.Preload("Creator").Where("id = ?", id), familyID)
	err := q.First(&r).Error
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// List 分页查询菜谱列表，支持关键词搜索、分类筛选和家庭过滤。
//
// 参数:
//   - familyID uint64 - 家庭 ID（> 0 时按家庭过滤，= 0 时不过滤）
//   - keyword  string - 搜索关键词（按菜名模糊匹配，为空时不过滤）
//   - category string - 分类筛选（如"家常菜"、"川菜"，为空时不过滤）
//   - page     int    - 页码（从 1 开始）
//   - pageSize int    - 每页条数
//
// 返回值:
//   - []model.Recipe - 当前页的菜谱列表（含 Creator 预加载）
//   - int64          - 符合条件的总记录数
//   - error          - 查询失败时返回错误
//
// GORM 操作:
//
//	第一步: db.Model(&Recipe{}).Where(...条件...).Count(&total)
//	  → SELECT COUNT(*) FROM recipes WHERE (family_id = ?) [AND name LIKE ?] [AND category = ?] AND deleted_at IS NULL
//	第二步: query.Order("created_at DESC").Offset(offset).Limit(pageSize).Preload("Creator").Find(&recipes)
//	  → SELECT * FROM recipes WHERE (...) ORDER BY created_at DESC LIMIT ? OFFSET ?
//	  → SELECT * FROM users WHERE id = ? (Preload Creator，每个菜谱一条)
//
// 说明:
//   - 条件为可选：familyID = 0 不过滤家庭，keyword/category 为空字符串时不过滤
//   - 按创建时间降序排列（最新菜谱在前）
//   - keyword 使用 LIKE '%keyword%' 模糊匹配
func (s *RecipeService) List(familyID uint64, keyword, category string, page, pageSize int) ([]model.Recipe, int64, error) {
	var recipes []model.Recipe
	var total int64

	query := applyRecipeReadScope(s.db.Model(&model.Recipe{}), familyID)
	if keyword != "" {
		// SQL LIKE 模糊匹配菜名（%keyword%）
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	// 统计符合条件的总记录数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 计算偏移量，执行分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).
		Preload("Creator").
		Find(&recipes).Error; err != nil {
		return nil, 0, err
	}
	return recipes, total, nil
}

// IncrementCookCount 增加指定菜谱的烹饪次数（原子递增 +1）。
//
// 参数:
//   - id uint64 - 菜谱 ID
//
// 返回值:
//   - error - 更新失败时返回错误
//
// GORM 操作:
//
//	db.Model(&Recipe{}).Where("id = ?", id).UpdateColumn("cook_count", gorm.Expr("cook_count + 1"))
//	→ UPDATE recipes SET cook_count = cook_count + 1, updated_at = updated_at WHERE id = ? AND deleted_at IS NULL
//
// 说明:
//   - 使用 gorm.Expr 构造 SQL 表达式，在数据库层面原子递增，避免并发读写竞争
//   - UpdateColumn 不更新 updated_at 时间戳（保持原有的更新时间语义）
//   - 通常在 recipe 被点菜并烹饪后调用，用于统计菜谱受欢迎程度
func (s *RecipeService) IncrementCookCount(id, familyID uint64) error {
	res := s.db.Model(&model.Recipe{}).Where("id = ? AND family_id = ?", id, familyID).
		UpdateColumn("cook_count", gorm.Expr("cook_count + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRecipeNotInFamily
	}
	return nil
}
