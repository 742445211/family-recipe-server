package service

import (
	"errors"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

type OrderService struct {
	db *gorm.DB
}

func NewOrderService(db *gorm.DB) *OrderService {
	return &OrderService{db: db}
}

// DB returns the underlying gorm.DB
func (s *OrderService) DB() *gorm.DB {
	return s.db
}

// Add 点一道菜到指定日期的指定餐次
func (s *OrderService) Add(familyID, recipeID uint64, mealType string, userID uint64, date, note string, quantity int) (*model.DailyOrder, error) {
	if quantity <= 0 {
		quantity = 1
	}

	// 同日期+同餐次+同菜不能重复（已删除的不算）
	var count int64
	s.db.Unscoped().Model(&model.DailyOrder{}).
		Where("family_id = ? AND date = ? AND meal_type = ? AND recipe_id = ? AND deleted_at IS NULL",
			familyID, date, mealType, recipeID).
		Count(&count)
	if count > 0 {
		return nil, errors.New("该餐次已点过这道菜")
	}

	order := model.DailyOrder{
		FamilyID: familyID,
		Date:     date,
		MealType: mealType,
		RecipeID: recipeID,
		AddedBy:  userID,
		Quantity: quantity,
		Note:     note,
	}
	if err := s.db.Create(&order).Error; err != nil {
		return nil, err
	}
	// 加载关联
	s.db.Preload("Recipe").Preload("Adder").First(&order, order.ID)
	return &order, nil
}

// Remove 取消点菜（硬删除 + 权限校验）
func (s *OrderService) Remove(orderID, userID uint64) error {
	result := s.db.Unscoped().Where("id = ? AND added_by = ?", orderID, userID).Delete(&model.DailyOrder{})
	if result.RowsAffected == 0 {
		return errors.New("无权删除或不存在")
	}
	return result.Error
}

// GetByDateAndMeal 获取某天某餐次的点菜列表，mealType 为空则返回当天全部
func (s *OrderService) GetByDateAndMeal(familyID uint64, date, mealType string) ([]model.DailyOrder, error) {
	var orders []model.DailyOrder
	q := s.db.Where("family_id = ? AND date = ?", familyID, date)
	if mealType != "" {
		q = q.Where("meal_type = ?", mealType)
	}
	err := q.Preload("Recipe").Preload("Adder").
		Order("meal_type, created_at ASC").
		Find(&orders).Error
	return orders, err
}

// GetRecentDates 获取最近 N 天的点菜日期
func (s *OrderService) GetRecentDates(familyID uint64, limit int) ([]string, error) {
	var dates []string
	err := s.db.Model(&model.DailyOrder{}).
		Where("family_id = ?", familyID).
		Order("date DESC").
		Limit(limit).
		Pluck("date", &dates).Error
	return dates, err
}
