// Package service - 每日点菜服务。
//
// 本文件实现 DailyOrder（每日点菜）的完整业务逻辑，是当前推荐使用的点菜方案。
// 与旧 Menu/MenuItem 方案相比，DailyOrder 按"日期 + 餐次"维度组织，更适合日常家庭场景。
//
// 核心功能：
//   - 添加点菜（同日期同餐次同菜不可重复）
//   - 取消点菜（软删除，仅点菜人本人可操作）
//   - 按日期/餐次查询点菜列表
//   - 获取最近点菜日期列表
//
// 说明：
//   - 所有删除操作使用 GORM 软删除（设置 deleted_at，不实际删除行）
//   - 查询自动过滤软删除记录（GORM 默认行为）
package service

import (
	"errors"
	"strings"
	"time"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

var (
	ErrDuplicateOrder  = errors.New("该餐次已点过这道菜")
	ErrInvalidMealType = errors.New("无效的餐次类型")
)

var validMealTypes = map[string]struct{}{
	"breakfast": {},
	"lunch":     {},
	"dinner":    {},
}

func normalizeMealTypeInput(mealType string) (string, error) {
	mealType = strings.TrimSpace(mealType)
	if mealType == "" {
		mealType = "dinner"
	}
	if _, ok := validMealTypes[mealType]; !ok {
		return "", ErrInvalidMealType
	}
	return mealType, nil
}

// OrderService 每日点菜服务，封装 DailyOrder 的数据库操作。
// 支持按日期 + 餐次维度的点菜增删查，以及授权管理（仅点菜人可删除）。
type OrderService struct {
	db *gorm.DB // GORM 数据库实例
}

// NewOrderService 创建每日点菜服务实例。
//
// 参数:
//   - db *gorm.DB - GORM 数据库连接
//
// 返回值:
//   - *OrderService - 点菜服务指针
func NewOrderService(db *gorm.DB) *OrderService {
	return &OrderService{db: db}
}

// DB 返回底层 GORM 数据库实例，供 handler 层在需要事务或其他直接数据库操作时使用。
//
// 返回值:
//   - *gorm.DB - GORM 数据库连接
func (s *OrderService) DB() *gorm.DB {
	return s.db
}

// Add 为指定家庭的指定日期+餐次添加一道菜（创建一条 DailyOrder 记录）。
//
// 参数:
//   - familyID uint64 - 家庭 ID
//   - recipeID uint64 - 菜谱 ID
//   - mealType string - 餐次类型：breakfast（早餐）/ lunch（午餐）/ dinner（晚餐）
//   - userID   uint64 - 点菜人用户 ID
//   - date     string - 日期（YYYY-MM-DD 格式）
//   - note     string - 备注（如"少放盐"）
//   - quantity int    - 数量（若传入 <= 0 则默认为 1）
//
// 返回值:
//   - *model.DailyOrder - 新创建的点菜记录（含预加载的 Recipe 和 Adder 关联）
//   - error             - 重复点菜或数据库操作失败时返回错误
//
// 错误情况:
//   - "该餐次已点过这道菜" - 同家庭同日期同餐次已存在该菜谱的点菜记录
//
// GORM 操作:
//
//	第一步: db.Model(&DailyOrder{}).Where("family_id = ? AND date = ? AND meal_type = ? AND recipe_id = ?").Count(&count)
//	  → SELECT COUNT(*) FROM daily_orders WHERE family_id=? AND date=? AND meal_type=? AND recipe_id=? AND deleted_at IS NULL
//	第二步: db.Create(&order)
//	  → INSERT INTO daily_orders (...) VALUES (...)
//	第三步: db.Preload("Recipe").Preload("Adder").First(&order, order.ID)
//	  → SELECT * FROM daily_orders WHERE id = ? (含关联预加载)
//
// 说明:
//   - 去重检查：同一家庭同日期同餐次不允许点同一道菜（软删除的记录不计入重复判断）
//   - 创建后立即预加载关联数据（Recipe、Adder），以便返回完整的点菜信息给前端
func (s *OrderService) Add(familyID, recipeID uint64, mealType string, userID uint64, date, note string, quantity int) (*model.DailyOrder, error) {
	if quantity <= 0 {
		quantity = 1
	}
	var err error
	mealType, err = normalizeMealTypeInput(mealType)
	if err != nil {
		return nil, err
	}

	var recipe model.Recipe
	if err := s.db.Where("id = ? AND (family_id = ? OR is_public = ?)", recipeID, familyID, true).
		First(&recipe).Error; err != nil {
		return nil, ErrRecipeNotInFamily
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

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.DailyOrder{}).
			Where("family_id = ? AND date = ? AND meal_type = ? AND recipe_id = ?",
				familyID, date, mealType, recipeID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrDuplicateOrder
		}
		return tx.Create(&order).Error
	})
	if err != nil {
		if errors.Is(err, ErrDuplicateOrder) {
			return nil, err
		}
		return nil, err
	}

	if err := s.db.Preload("Recipe").Preload("Adder").First(&order, order.ID).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// Remove 取消点菜（软删除），仅允许点菜人本人操作。
//
// 参数:
//   - orderID uint64 - 点菜记录 ID
//   - userID  uint64 - 操作人用户 ID
//
// 返回值:
//   - error - 无权删除、记录不存在或数据库操作失败时返回错误
//
// 错误情况:
//   - "无权删除或不存在" - 记录不属于当前用户，或记录已不存在
//
// GORM 操作:
//
//	db.Where("id = ? AND added_by = ?", orderID, userID).Delete(&model.DailyOrder{})
//	→ UPDATE daily_orders SET deleted_at = NOW() WHERE id = ? AND added_by = ? AND deleted_at IS NULL
//
// 说明:
//   - GORM 的 Delete 对软删除模型执行的是 UPDATE 而非物理 DELETE
//   - 通过 Where 条件中的 added_by = userID 保证仅点菜人可以删除
//   - RowsAffected == 0 表示记录不存在或不属于该用户
func (s *OrderService) Remove(orderID, familyID, userID uint64) error {
	result := s.db.Where("id = ? AND family_id = ? AND added_by = ?", orderID, familyID, userID).
		Delete(&model.DailyOrder{})
	if result.RowsAffected == 0 {
		return errors.New("无权删除或不存在")
	}
	return result.Error
}

// GetByDateAndMeal 查询指定家庭某天某个餐次的点菜列表。
//
// 参数:
//   - familyID uint64 - 家庭 ID
//   - date     string - 日期（YYYY-MM-DD）
//   - mealType string - 餐次类型，为空则返回当天所有餐次的点菜
//
// 返回值:
//   - []model.DailyOrder - 点菜列表（含 Recipe 和 Adder 预加载）
//   - error              - 查询失败时返回错误
//
// GORM 操作:
//
//	db.Where("family_id = ? AND date = ?", familyID, date)
//	  .Where("meal_type = ?", mealType)  // mealType 非空时追加
//	  .Preload("Recipe").Preload("Adder")
//	  .Order("meal_type, created_at ASC")
//	  .Find(&orders)
//	→ SELECT * FROM daily_orders WHERE family_id=? AND date=? [AND meal_type=?] AND deleted_at IS NULL ORDER BY meal_type, created_at ASC
//
// 说明:
//   - 结果按 meal_type（早餐→午餐→晚餐）再按创建时间升序排列
//   - mealType 为空时返回当天全部餐次的点菜，方便前端做日视图展示
func (s *OrderService) GetByDateAndMeal(familyID uint64, date, mealType string) ([]model.DailyOrder, error) {
	var orders []model.DailyOrder

	// 基础查询：家庭 + 日期
	q := s.db.Where("family_id = ? AND date = ?", familyID, date)

	// 如果指定了餐次，追加餐次过滤条件
	if mealType != "" {
		q = q.Where("meal_type = ?", mealType)
	}

	// 预加载关联、排序、执行查询
	err := q.Preload("Recipe").Preload("Adder").
		Order("meal_type, created_at ASC").
		Find(&orders).Error
	return orders, err
}

// GetRecentDates 获取指定家庭最近 N 天有点菜记录的日期列表（去重）。
//
// 参数:
//   - familyID uint64 - 家庭 ID
//   - limit    int    - 返回的最大日期数量
//
// 返回值:
//   - []string - 日期字符串列表（YYYY-MM-DD 格式，按日期倒序）
//   - error    - 查询失败时返回错误
//
// GORM 操作:
//
//	db.Model(&DailyOrder{}).Where("family_id = ?").Order("date DESC").Limit(limit).Pluck("date", &dates)
//	→ SELECT DISTINCT date FROM daily_orders WHERE family_id=? AND deleted_at IS NULL ORDER BY date DESC LIMIT ?
//
// 说明:
//   - Pluck 直接提取 date 列到 []string 切片，不做结构体映射
//   - 结果按日期降序（最近日期在前），前端可用于构建日期选择器
// GetRecentOrderNames 近 N 天内最近点菜菜名（最多 maxMeals 条，按日期倒序）。
func (s *OrderService) GetRecentOrderNames(familyID uint64, days int, maxMeals int) ([]string, error) {
	if days <= 0 {
		days = 7
	}
	if maxMeals <= 0 {
		maxMeals = 21
	}
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var orders []model.DailyOrder
	err := s.db.Preload("Recipe").
		Where("family_id = ? AND date >= ?", familyID, since).
		Order("date DESC, CASE meal_type WHEN 'dinner' THEN 3 WHEN 'lunch' THEN 2 ELSE 1 END DESC, created_at DESC").
		Limit(maxMeals).
		Find(&orders).Error
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(orders))
	for _, o := range orders {
		if o.Recipe != nil && o.Recipe.Name != "" {
			names = append(names, o.Recipe.Name)
		}
	}
	return names, nil
}

func (s *OrderService) GetRecentDates(familyID uint64, limit int) ([]string, error) {
	var dates []string
	err := s.db.Model(&model.DailyOrder{}).
		Where("family_id = ?", familyID).
		Order("date DESC").
		Limit(limit).
		Pluck("date", &dates).Error
	return dates, err
}
