package service

import (
	"fmt"
	"strings"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建内存 SQLite 用于测试
func setupTestDB(t *testing.T) *gorm.DB {
	// 每用例独立内存库；cache=shared + MaxOpenConns(1) 供异步通知 goroutine 复用连接
	safeName := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := fmt.Sprintf("file:memdb_%s?mode=memory&cache=shared", safeName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	db.AutoMigrate(
		&model.Family{},
		&model.User{},
		&model.FamilyMember{},
		&model.Recipe{},
		&model.DailyOrder{},
		&model.Favorite{},
		&model.Notification{},
		&model.NotificationDelivery{},
		&model.NotificationChannel{},
	)
	return db
}

func ensureAppConfig() {
	if config.AppConfig == nil {
		_ = config.Load("../../config.yaml")
	}
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
	}
	if config.AppConfig.JWT.Secret == "" {
		config.AppConfig.JWT = config.JWTConfig{Secret: "test-secret", ExpireHours: 24}
	}
}

// initTestConfig 测试默认开启通知（不依赖生产 config.yaml 是否含 notification 段）。
func initTestConfig() {
	initTestConfigNotification(true)
}

// initTestConfigNotification 按指定开关初始化通知测试配置。
func initTestConfigNotification(enabled bool) {
	ensureAppConfig()
	applyTestNotificationConfig(enabled)
}

func applyTestNotificationConfig(enabled bool) {
	n := &config.AppConfig.Notification
	n.Enabled = enabled
	n.Worker.Enabled = false
	if !enabled {
		return
	}
	if !n.WebSocket.Enabled {
		n.WebSocket.Enabled = true
	}
	if n.WeChatSubscribe.TemplateID == "" {
		n.WeChatSubscribe.Enabled = true
		n.WeChatSubscribe.TemplateID = "test-template"
	}
	if config.AppConfig.WeChat.AppID == "" {
		config.AppConfig.WeChat.AppID = "test"
	}
	if config.AppConfig.WeChat.Secret == "" {
		config.AppConfig.WeChat.Secret = "test"
	}
}

func requireNotificationEnabled(t *testing.T) {
	t.Helper()
	if config.AppConfig == nil || !config.AppConfig.Notification.Enabled {
		t.Skip("notification.enabled=false，跳过需通知开启的用例")
	}
}

func seedUserAndFamily(t *testing.T, db *gorm.DB) (uint64, uint64) {
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

func TestRecipeCRUD(t *testing.T) {
	db := setupTestDB(t)
	_, familyID := seedUserAndFamily(t, db)
	svc := NewRecipeService(db)

	// Create
	r := &model.Recipe{
		Name:       "番茄炒蛋",
		Category:   "荤菜",
		Difficulty: "easy",
		CookTime:   15,
		CreatorID:  1,
		FamilyID:   familyID,
		Ingredients: `[{"name":"番茄","amount":"2个"},{"name":"鸡蛋","amount":"3个"}]`,
		Steps:      `["番茄切块","鸡蛋打散","先炒蛋再加番茄"]`,
	}
	if err := svc.Create(r); err != nil {
		t.Fatalf("创建菜谱失败: %v", err)
	}
	if r.ID == 0 {
		t.Fatal("创建后 ID 应为非零")
	}

	// GetByID
	got, err := svc.GetByID(r.ID)
	if err != nil {
		t.Fatalf("获取菜谱失败: %v", err)
	}
	if got.Name != "番茄炒蛋" {
		t.Errorf("菜名: want 番茄炒蛋, got %s", got.Name)
	}

	// Update
	if err := svc.Update(&model.Recipe{ID: r.ID, Name: "番茄炒蛋升级版"}); err != nil {
		t.Fatalf("更新菜谱失败: %v", err)
	}
	got, _ = svc.GetByID(r.ID)
	if got.Name != "番茄炒蛋升级版" {
		t.Errorf("更新后菜名: want 番茄炒蛋升级版, got %s", got.Name)
	}

	// List
	list, total, err := svc.List(familyID, "", "", 1, 10)
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if total != 1 {
		t.Errorf("总数: want 1, got %d", total)
	}
	if len(list) != 1 {
		t.Errorf("列表长度: want 1, got %d", len(list))
	}

	// Search
	list, total, _ = svc.List(familyID, "番茄", "", 1, 10)
	if total != 1 {
		t.Errorf("搜索番茄: want 1, got %d", total)
	}
	list, total, _ = svc.List(familyID, "不存在", "", 1, 10)
	if total != 0 {
		t.Errorf("搜索不存在: want 0, got %d", total)
	}

	// CookCount
	svc.IncrementCookCount(r.ID)
	got, _ = svc.GetByID(r.ID)
	if got.CookCount != 1 {
		t.Errorf("做过次数: want 1, got %d", got.CookCount)
	}

	// Delete
	if err := svc.Delete(r.ID, 1); err != nil {
		t.Fatalf("删除菜谱失败: %v", err)
	}
	_, err = svc.GetByID(r.ID)
	if err == nil {
		t.Fatal("删除后仍能查到菜谱")
	}
}
