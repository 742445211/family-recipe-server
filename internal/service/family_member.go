package service

import (
	"errors"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

var (
	ErrNotFamilyMember = errors.New("不是该家庭的成员")
	ErrNoFamily        = errors.New("请先加入家庭")
)

// IsFamilyMember 判断用户是否为家庭成员。
func IsFamilyMember(db *gorm.DB, userID, familyID uint64) bool {
	if userID == 0 || familyID == 0 {
		return false
	}
	var count int64
	db.Model(&model.FamilyMember{}).
		Where("family_id = ? AND user_id = ?", familyID, userID).
		Count(&count)
	return count > 0
}

// ResolveJWTFamilyID 校验 JWT 中的 family_id；非成员则返回 0。
func ResolveJWTFamilyID(db *gorm.DB, userID, claimedFamilyID uint64) uint64 {
	if claimedFamilyID == 0 {
		return 0
	}
	if IsFamilyMember(db, userID, claimedFamilyID) {
		return claimedFamilyID
	}
	return 0
}

// ResolveEffectiveFamilyID 解析请求上下文应使用的家庭 ID：
// 优先 JWT 声明（须为成员）；声明为 0 时回退 users.current_family_id（兼容未换 token 的客户端）。
func ResolveEffectiveFamilyID(db *gorm.DB, userID, claimedFamilyID uint64) uint64 {
	if fid := ResolveJWTFamilyID(db, userID, claimedFamilyID); fid > 0 {
		return fid
	}
	if userID == 0 || db == nil {
		return 0
	}
	var user model.User
	if err := db.Select("current_family_id").First(&user, userID).Error; err != nil {
		return 0
	}
	if user.CurrentFamilyID == nil {
		return 0
	}
	return ResolveJWTFamilyID(db, userID, *user.CurrentFamilyID)
}

// AssertFamilyMember 校验用户属于指定家庭。
func AssertFamilyMember(db *gorm.DB, userID, familyID uint64) error {
	if familyID == 0 {
		return errors.New("请先加入家庭")
	}
	if !IsFamilyMember(db, userID, familyID) {
		return ErrNotFamilyMember
	}
	return nil
}
