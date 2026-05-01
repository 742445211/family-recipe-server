package service

import (
	"errors"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

type MenuService struct {
	db *gorm.DB
}

func NewMenuService(db *gorm.DB) *MenuService {
	return &MenuService{db: db}
}

// Create 创建点菜单
func (s *MenuService) Create(m *model.Menu) error {
	return s.db.Create(m).Error
}

// GetByID 获取点菜单详情
func (s *MenuService) GetByID(id uint64) (*model.Menu, error) {
	var m model.Menu
	err := s.db.Preload("Creator").
		Preload("Items.Recipe").
		Preload("Items.Adder").
		First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// List 菜单列表
func (s *MenuService) List(familyID uint64, page, pageSize int) ([]model.Menu, int64, error) {
	var menus []model.Menu
	var total int64

	query := s.db.Model(&model.Menu{}).Where("family_id = ?", familyID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).
		Preload("Creator").
		Find(&menus).Error; err != nil {
		return nil, 0, err
	}
	return menus, total, nil
}

// AddItem 往菜单加菜
func (s *MenuService) AddItem(menuID, recipeID, userID uint64, quantity int, note string) (*model.MenuItem, error) {
	// 检查菜单是否存在
	var menu model.Menu
	if err := s.db.First(&menu, menuID).Error; err != nil {
		return nil, errors.New("菜单不存在")
	}
	if menu.Status == "confirmed" {
		return nil, errors.New("菜单已确认，无法修改")
	}

	item := model.MenuItem{
		MenuID:   menuID,
		RecipeID: recipeID,
		AddedBy:  userID,
		Quantity: quantity,
		Note:     note,
	}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// RemoveItem 删除点菜项
func (s *MenuService) RemoveItem(itemID, userID uint64) error {
	var item model.MenuItem
	if err := s.db.First(&item, itemID).Error; err != nil {
		return errors.New("点菜项不存在")
	}
	// 只能删除自己点的，或者菜单确认后不能删
	var menu model.Menu
	if err := s.db.First(&menu, item.MenuID).Error; err != nil {
		return err
	}
	if menu.Status == "confirmed" {
		return errors.New("菜单已确认，无法修改")
	}
	return s.db.Where("id = ?", itemID).Delete(&model.MenuItem{}).Error
}

// ConfirmMenu 确认菜单
func (s *MenuService) ConfirmMenu(menuID uint64) error {
	return s.db.Model(&model.Menu{}).Where("id = ?", menuID).
		Update("status", "confirmed").Error
}
