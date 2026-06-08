// Package testutil 提供 service / handler 层测试共用的内存数据库与种子数据。
package testutil

import (
	"fmt"
	"strings"
	"testing"

	"recipe-server/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SetupTestDB 创建内存 SQLite 用于测试。
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	safeName := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := fmt.Sprintf("file:memdb_%s?mode=memory&cache=shared", safeName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(
		&model.Family{},
		&model.User{},
		&model.FamilyMember{},
		&model.Recipe{},
		&model.DailyOrder{},
		&model.Favorite{},
		&model.Menu{},
		&model.MenuItem{},
		&model.Notification{},
		&model.NotificationDelivery{},
		&model.NotificationChannel{},
	); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	return db
}

// SeedUserAndFamily 创建测试用户与家庭并返回 userID、familyID。
func SeedUserAndFamily(t *testing.T, db *gorm.DB) (uint64, uint64) {
	t.Helper()
	user := model.User{OpenID: "test-openid", Nickname: "测试用户"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	family := model.Family{Name: "测试家庭", InviteCode: "TEST01"}
	if err := db.Create(&family).Error; err != nil {
		t.Fatalf("创建测试家庭失败: %v", err)
	}
	member := model.FamilyMember{FamilyID: family.ID, UserID: user.ID, Role: "owner"}
	db.Create(&member)
	db.Model(&user).Update("current_family_id", family.ID)
	return user.ID, family.ID
}

// SeedChefOrder 创建含厨师的点菜记录，返回 orderID、chefUserID。
func SeedChefOrder(t *testing.T, db *gorm.DB) (orderID uint64, chefID uint64) {
	t.Helper()

	adder := model.User{OpenID: "adder-oid", Nickname: "点菜人"}
	chef := model.User{OpenID: "chef-oid", Nickname: "厨师"}
	db.Create(&adder)
	db.Create(&chef)

	family := model.Family{Name: "测试家", InviteCode: "CHEF01"}
	db.Create(&family)
	db.Create(&model.FamilyMember{FamilyID: family.ID, UserID: adder.ID, Role: "member"})
	db.Create(&model.FamilyMember{FamilyID: family.ID, UserID: chef.ID, Role: "member", IsChef: true})

	recipe := model.Recipe{Name: "红烧肉", Category: "荤菜", CreatorID: adder.ID, FamilyID: family.ID}
	db.Create(&recipe)

	order := model.DailyOrder{
		FamilyID: family.ID, Date: "2026-06-05", MealType: "dinner",
		RecipeID: recipe.ID, AddedBy: adder.ID, Quantity: 1,
	}
	db.Create(&order)
	return order.ID, chef.ID
}
