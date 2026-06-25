package middleware

import (
	"net/http"
	"strings"

	"recipe-server/config"
	"recipe-server/internal/service"
	"recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetUserID 从 Gin context 中获取当前登录用户 ID；未登录时返回 0。
func GetUserID(c *gin.Context) uint64 {
	id, ok := c.Get("user_id")
	if !ok {
		return 0
	}
	v, _ := id.(uint64)
	return v
}

// GetFamilyID 从 Gin context 中获取当前用户所属家庭 ID；未登录或未加入家庭时返回 0。
func GetFamilyID(c *gin.Context) uint64 {
	id, ok := c.Get("family_id")
	if !ok {
		return 0
	}
	v, _ := id.(uint64)
	return v
}

func injectClaims(c *gin.Context, db *gorm.DB, claims *jwt.Claims) {
	c.Set("user_id", claims.UserID)
	c.Set("openid", claims.OpenID)
	familyID := claims.FamilyID
	if db != nil {
		familyID = service.ResolveEffectiveFamilyID(db, claims.UserID, claims.FamilyID)
	}
	c.Set("family_id", familyID)
}

// AuthRequired 返回 JWT 认证中间件；db 非 nil 时校验 JWT family_id 的成员关系。
func AuthRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
			c.Abort()
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "认证格式错误"})
			c.Abort()
			return
		}

		claims, err := jwt.Parse(config.AppConfig.JWT.Secret, parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Token无效或已过期"})
			c.Abort()
			return
		}

		injectClaims(c, db, claims)
		c.Next()
	}
}

// OptionalAuth 可选 JWT 认证：有合法 Bearer token 时注入用户信息，否则继续处理。
func OptionalAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.Next()
			return
		}
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}
		claims, err := jwt.Parse(config.AppConfig.JWT.Secret, parts[1])
		if err != nil {
			c.Next()
			return
		}
		injectClaims(c, db, claims)
		c.Next()
	}
}
