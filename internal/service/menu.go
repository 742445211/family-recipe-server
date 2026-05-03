// Package service - 点菜单管理服务。
//
// 本文件实现点菜单（Menu + MenuItem）的完整生命周期管理，包括：
//   - 菜单的创建与查询
//   - 菜单项（菜品）的添加与删除
//   - 菜单确认（状态流转：draft → confirmed）
//
// 业务规则：
//   - 菜单在确认后不可修改（items 不可增删）
//   - 同一菜单内不允许重复点同一道菜（由 MenuItem 的 uk_menu_recipe 唯一索引保证）
//
// 注意：Menu/MenuItem 为早期菜单方案，新业务优先使用 DailyOrder（见 order.go）。
// Menu 当前仍保留用于兼容旧数据和批量菜单场景。
package service

import (
	"errors"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

// MenuService 点菜单服务，封装 Menu 和 MenuItem 的数据库操作。
// 通过 GORM 操作 menus 和 menu_items 两张表，支持关联预加载。
type MenuService struct {
	db *gorm.DB // GORM 数据库实例
}

// NewMenuService 创建点菜单服务实例。
//
// 参数:
//   - db *gorm.DB - GORM 数据库连接
//
// 返回值:
//   - *MenuService - 点菜单服务指针
func NewMenuService(db *gorm.DB) *MenuService {
	return &MenuService{db: db}
}

// Create 创建一条新的点菜单记录。
//
// 参数:
//   - m *model.Menu - 待创建的菜单对象（需包含 FamilyID、Name、Date、CreatorID 等必填字段）
//
// 返回值:
//   - error - GORM INSERT 操作失败时返回错误
//
// GORM 操作:
//
//	db.Create(m) → INSERT INTO menus (...) VALUES (...)
func (s *MenuService) Create(m *model.Menu) error {
	return s.db.Create(m).Error
}

// GetByID 根据 ID 获取点菜单详情，同时预加载关联数据。
//
// 参数:
//   - id uint64 - 菜单主键 ID
//
// 返回值:
//   - *model.Menu - 菜单对象（含 Creator、Items.Recipe、Items.Adder 预加载）
//   - error       - 记录不存在时返回 gorm.ErrRecordNotFound，其他查询错误也返回
//
// GORM 操作:
//
//	db.Preload("Creator").Preload("Items.Recipe").Preload("Items.Adder").First(&m, id)
//	→ SELECT * FROM menus WHERE id = ? AND deleted_at IS NULL LIMIT 1
//	→ SELECT * FROM menu_items WHERE menu_id = ? AND deleted_at IS NULL (Preload Items)
//	→ SELECT * FROM recipes WHERE id = ? (Preload Items.Recipe)
//	→ SELECT * FROM users WHERE id = ? (Preload Items.Adder / Creator)
//
// 说明:
//   - Preload 为 GORM 的预加载机制，一次性加载菜单 → 菜单项 → 菜谱/点菜人的嵌套关联
//   - First 自动附带软删除条件（deleted_at IS NULL）
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

// List 分页查询指定家庭的菜单列表。
//
// 参数:
//   - familyID uint64 - 家庭 ID
//   - page     int    - 页码（从 1 开始）
//   - pageSize int    - 每页条数
//
// 返回值:
//   - []model.Menu - 当前页的菜单列表（含 Creator 预加载）
//   - int64        - 符合条件的总记录数
//   - error        - 查询失败时返回错误
//
// GORM 操作:
//
//	第一步: db.Model(&Menu{}).Where("family_id = ?").Count(&total)
//	  → SELECT COUNT(*) FROM menus WHERE family_id = ? AND deleted_at IS NULL
//	第二步: query.Order("created_at DESC").Offset(offset).Limit(pageSize).Preload("Creator").Find(&menus)
//	  → SELECT * FROM menus WHERE family_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?
//	  → SELECT * FROM users WHERE id = ? (Preload Creator)
//
// 说明:
//   - 按创建时间倒序排列（最新菜单在前）
//   - 分页使用 LIMIT + OFFSET 标准模式
func (s *MenuService) List(familyID uint64, page, pageSize int) ([]model.Menu, int64, error) {
	var menus []model.Menu
	var total int64

	// 构建基础查询：按家庭过滤
	query := s.db.Model(&model.Menu{}).Where("family_id = ?", familyID)

	// 先统计符合条件的总记录数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 计算偏移量，执行分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).
		Preload("Creator").
		Find(&menus).Error; err != nil {
		return nil, 0, err
	}
	return menus, total, nil
}

// AddItem 向菜单中添加一道菜（创建一个 MenuItem）。
//
// 参数:
//   - menuID   uint64 - 目标菜单 ID
//   - recipeID uint64 - 菜谱 ID
//   - userID   uint64 - 点菜人用户 ID
//   - quantity int    - 数量
//   - note     string - 备注（如"少放盐"）
//
// 返回值:
//   - *model.MenuItem - 新创建的菜单项（含自增 ID）
//   - error           - 菜单不存在、菜单已确认、或数据库插入失败时返回错误
//
// GORM 操作:
//
//	db.First(&menu, menuID) → 检查菜单存在性及状态
//	db.Create(&item)         → INSERT INTO menu_items (...) VALUES (...)
//
// 错误情况:
//   - "菜单不存在"           - 目标菜单 ID 不匹配或已被软删除
//   - "菜单已确认，无法修改" - menu.Status == "confirmed"
func (s *MenuService) AddItem(menuID, recipeID, userID uint64, quantity int, note string) (*model.MenuItem, error) {
	// 校验：检查菜单是否存在
	var menu model.Menu
	if err := s.db.First(&menu, menuID).Error; err != nil {
		return nil, errors.New("菜单不存在")
	}

	// 校验：已确认的菜单不允许修改
	if menu.Status == "confirmed" {
		return nil, errors.New("菜单已确认，无法修改")
	}

	// 构造并插入菜单项
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

// RemoveItem 删除菜单中的一道菜（物理删除 MenuItem）。
//
// 参数:
//   - itemID uint64 - 菜单项 ID
//   - userID uint64 - 操作人用户 ID（当前未做权限校验，保留参数供未来扩展）
//
// 返回值:
//   - error - 菜单项不存在、菜单已确认、或数据库删除失败时返回错误
//
// GORM 操作:
//
//	db.First(&item, itemID)   → 检查菜单项存在性
//	db.First(&menu, item.MenuID) → 检查关联菜单的状态
//	db.Where("id = ?", itemID).Delete(&model.MenuItem{}) → DELETE FROM menu_items WHERE id = ?
//
// 错误情况:
//   - "点菜项不存在"         - 目标项 ID 不匹配或已被删除
//   - "菜单已确认，无法修改" - 菜单已确认
func (s *MenuService) RemoveItem(itemID, userID uint64) error {
	// 校验：查找菜单项是否存在
	var item model.MenuItem
	if err := s.db.First(&item, itemID).Error; err != nil {
		return errors.New("点菜项不存在")
	}

	// 校验：查找关联菜单并检查状态，已确认的菜单不可修改
	var menu model.Menu
	if err := s.db.First(&menu, item.MenuID).Error; err != nil {
		return err
	}
	if menu.Status == "confirmed" {
		return errors.New("菜单已确认，无法修改")
	}

	// 执行物理删除
	return s.db.Where("id = ?", itemID).Delete(&model.MenuItem{}).Error
}

// ConfirmMenu 确认菜单，将菜单状态从 draft 变更为 confirmed。
//
// 参数:
//   - menuID uint64 - 菜单 ID
//
// 返回值:
//   - error - 查询或更新失败时返回错误
//
// GORM 操作:
//
//	db.Model(&Menu{}).Where("id = ?", menuID).Update("status", "confirmed")
//	→ UPDATE menus SET status = 'confirmed', updated_at = NOW() WHERE id = ? AND deleted_at IS NULL
//
// 说明:
//   - 使用 Model().Where().Update() 链式调用，只更新 status 单字段
//   - 确认后的菜单前端应禁止修改（items 增删由 AddItem/RemoveItem 自行拦截）
func (s *MenuService) ConfirmMenu(menuID uint64) error {
	return s.db.Model(&model.Menu{}).Where("id = ?", menuID).
		Update("status", "confirmed").Error
}
