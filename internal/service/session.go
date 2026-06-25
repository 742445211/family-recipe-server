package service

import (
	"recipe-server/config"
	"recipe-server/internal/model"
	jwtPkg "recipe-server/pkg/jwt"

	"gorm.io/gorm"
)

// IssueUserJWT 为已登录用户签发 JWT（family_id 经成员关系校验）。
func IssueUserJWT(db *gorm.DB, userID uint64) (string, error) {
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		return "", err
	}
	familyID := uint64(0)
	if user.CurrentFamilyID != nil {
		familyID = ResolveJWTFamilyID(db, userID, *user.CurrentFamilyID)
	}
	return jwtPkg.Generate(
		config.AppConfig.JWT.Secret,
		config.AppConfig.JWT.ExpireHours,
		user.ID,
		user.OpenID,
		familyID,
	)
}
