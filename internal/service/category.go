// Package service - 家庭菜谱分类维护。
//
// 分类与 recipes.category 字段对应；Ensure 在创建/更新菜谱时自动补全，
// SyncFromRecipes / SyncAllFamilies 从已有菜谱反向同步分类表。
package service

import (
	"sort"
	"strings"
	"unicode/utf8"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

// CategoryService 家庭菜谱分类维护。
type CategoryService struct {
	db *gorm.DB
}

func NewCategoryService(db *gorm.DB) *CategoryService {
	return &CategoryService{db: db}
}

// NormalizeCategoryName 归一化分类名（空值→其他，截断至 50 字符）。
func NormalizeCategoryName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "其他"
	}
	if utf8.RuneCountInString(name) > 50 {
		runes := []rune(name)
		name = string(runes[:50])
	}
	return name
}

// Ensure 确保分类存在于当前家庭，不存在则新增；返回归一化后的名称。
func (s *CategoryService) Ensure(familyID uint64, name string) (string, error) {
	name = NormalizeCategoryName(name)
	if familyID == 0 {
		return name, nil
	}
	cat := model.RecipeCategory{FamilyID: familyID, Name: name}
	if err := s.db.Where("family_id = ? AND name = ?", familyID, name).
		FirstOrCreate(&cat).Error; err != nil {
		return "", err
	}
	return name, nil
}

// List 返回家庭分类列表。
func (s *CategoryService) List(familyID uint64) ([]model.RecipeCategory, error) {
	var cats []model.RecipeCategory
	err := s.db.Where("family_id = ?", familyID).
		Order("sort_order ASC, name ASC").
		Find(&cats).Error
	return cats, err
}

// ListNames 返回家庭分类名称列表。
func (s *CategoryService) ListNames(familyID uint64) ([]string, error) {
	cats, err := s.List(familyID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		out = append(out, c.Name)
	}
	return out, nil
}

// SyncFromRecipes 将家庭菜谱里已有 category 同步到分类表。
func (s *CategoryService) SyncFromRecipes(familyID uint64) error {
	var names []string
	if err := s.db.Model(&model.Recipe{}).
		Where("family_id = ?", familyID).
		Distinct("category").
		Pluck("category", &names).Error; err != nil {
		return err
	}
	for _, n := range names {
		if _, err := s.Ensure(familyID, n); err != nil {
			return err
		}
	}
	return nil
}

// ListPublicNames 返回所有公开菜谱中出现过的分类名（去重、归一化、按名称排序）。
func (s *CategoryService) ListPublicNames() ([]string, error) {
	var raw []string
	if err := s.db.Model(&model.Recipe{}).
		Where("is_public = ?", true).
		Distinct("category").
		Pluck("category", &raw).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, n := range raw {
		n = NormalizeCategoryName(n)
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// SyncAllFamilies 从全库菜谱同步分类（启动时补齐历史数据）。
func (s *CategoryService) SyncAllFamilies() error {
	var familyIDs []uint64
	if err := s.db.Model(&model.Recipe{}).Distinct("family_id").Pluck("family_id", &familyIDs).Error; err != nil {
		return err
	}
	for _, fid := range familyIDs {
		if err := s.SyncFromRecipes(fid); err != nil {
			return err
		}
	}
	return nil
}
