package service

import (
	"recipe-server/internal/model"

	"gorm.io/gorm"
)

type RecipeService struct {
	db *gorm.DB
}

func NewRecipeService(db *gorm.DB) *RecipeService {
	return &RecipeService{db: db}
}

// Create 创建菜谱
func (s *RecipeService) Create(r *model.Recipe) error {
	return s.db.Create(r).Error
}

// Update 更新菜谱
func (s *RecipeService) Update(r *model.Recipe) error {
	return s.db.Model(&model.Recipe{}).Where("id = ?", r.ID).Updates(r).Error
}

// Delete 删除菜谱
func (s *RecipeService) Delete(id, userID uint64) error {
	return s.db.Where("id = ? AND creator_id = ?", id, userID).Delete(&model.Recipe{}).Error
}

// GetByID 获取菜谱详情
func (s *RecipeService) GetByID(id uint64) (*model.Recipe, error) {
	var r model.Recipe
	err := s.db.Preload("Creator").First(&r, id).Error
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// List 菜谱列表（支持关键词搜索、分类筛选）
func (s *RecipeService) List(familyID uint64, keyword, category string, page, pageSize int) ([]model.Recipe, int64, error) {
	var recipes []model.Recipe
	var total int64

	query := s.db.Model(&model.Recipe{}).Where("family_id = ?", familyID)
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Preload("Creator").Find(&recipes).Error; err != nil {
		return nil, 0, err
	}
	return recipes, total, nil
}

// IncrementCookCount 增加烹饪次数
func (s *RecipeService) IncrementCookCount(id uint64) error {
	return s.db.Model(&model.Recipe{}).Where("id = ?", id).
		UpdateColumn("cook_count", gorm.Expr("cook_count + 1")).Error
}
