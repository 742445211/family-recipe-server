package service

import (
	"errors"

	"recipe-server/internal/model"

	"gorm.io/gorm"
)

var ErrNotFamilyMember = errors.New("不是该家庭的成员")

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
