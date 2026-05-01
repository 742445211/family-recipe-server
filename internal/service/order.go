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

// Add 点一道菜到今日
func (s *OrderService) Add(familyID, recipeID, userID uint64, date, note string, quantity int) (*model.DailyOrder, error) {
	if quantity <= 0 {
		quantity = 1
	}
	order := model.DailyOrder{
		FamilyID: familyID,
		Date:     date,
		RecipeID: recipeID,
		AddedBy:  userID,
		Quantity: quantity,
		Note:     note,
	}
	if err := s.db.Create(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// Remove 取消一道点菜
func (s *OrderService) Remove(orderID, userID uint64) error {
	result := s.db.Where("id = ? AND added_by = ?", orderID, userID).Delete(&model.DailyOrder{})
	if result.RowsAffected == 0 {
		return errors.New("无权删除或不存在")
	}
	return result.Error
}

// GetByDate 获取某天的点菜列表
func (s *OrderService) GetByDate(familyID uint64, date string) ([]model.DailyOrder, error) {
	var orders []model.DailyOrder
	err := s.db.Where("family_id = ? AND date = ?", familyID, date).
		Preload("Recipe").Preload("Adder").
		Order("created_at ASC").
		Find(&orders).Error
	return orders, err
}

// GetRecent 获取最近 N 天的点菜日期（用于日历选择）
func (s *OrderService) GetRecentDates(familyID uint64, limit int) ([]string, error) {
	var dates []string
	err := s.db.Model(&model.DailyOrder{}).
		Where("family_id = ?", familyID).
		Order("date DESC").
		Limit(limit).
		Pluck("date", &dates).Error
	return dates, err
}
