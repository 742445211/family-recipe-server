package service

import (
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestFamilyCreateAndJoin(t *testing.T) {
	db := testutil.SetupTestDB(t)

	// 创建两个用户
	u1 := model.User{OpenID: "user-1", Nickname: "用户1"}
	u2 := model.User{OpenID: "user-2", Nickname: "用户2"}
	db.Create(&u1)
	db.Create(&u2)

	// 用户1创建家庭
	family := model.Family{Name: "测试小家", InviteCode: "ABC123"}
	if err := db.Create(&family).Error; err != nil {
		t.Fatalf("创建家庭失败: %v", err)
	}
	if family.InviteCode != "ABC123" {
		t.Errorf("邀请码: want ABC123, got %s", family.InviteCode)
	}

	// 创建者为owner
	member1 := model.FamilyMember{FamilyID: family.ID, UserID: u1.ID, Role: "owner"}
	db.Create(&member1)

	// 用户2通过邀请码加入
	member2 := model.FamilyMember{FamilyID: family.ID, UserID: u2.ID, Role: "member"}
	if err := db.Where(model.FamilyMember{FamilyID: family.ID, UserID: u2.ID}).
		FirstOrCreate(&member2).Error; err != nil {
		t.Fatalf("加入家庭失败: %v", err)
	}
	if member2.Role != "member" {
		t.Errorf("新成员角色: want member, got %s", member2.Role)
	}

	// 查询家庭成员
	var members []model.FamilyMember
	db.Where("family_id = ?", family.ID).Preload("User").Find(&members)
	if len(members) != 2 {
		t.Errorf("家庭成员数: want 2, got %d", len(members))
	}

	// 查询用户所在家庭
	var myMembers []model.FamilyMember
	db.Where("user_id = ?", u1.ID).Preload("Family").Find(&myMembers)
	if len(myMembers) != 1 {
		t.Errorf("用户1的家庭数: want 1, got %d", len(myMembers))
	}

	// 重复加入不应创建重复记录
	db.Where(model.FamilyMember{FamilyID: family.ID, UserID: u2.ID}).FirstOrCreate(&member2)
	var memberCount int64
	db.Model(&model.FamilyMember{}).Where("family_id = ?", family.ID).Count(&memberCount)
	if memberCount != 2 {
		t.Errorf("重复加入后成员数: want 2, got %d", memberCount)
	}
}
