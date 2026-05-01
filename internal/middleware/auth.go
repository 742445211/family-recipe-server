package middleware

import (
	"net/http"
	"strings"

	"recipe-server/config"
	"recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
)

// GetUserID 从 context 获取当前用户ID
func GetUserID(c *gin.Context) uint64 {
	id, _ := c.Get("user_id")
	return id.(uint64)
}

// GetFamilyID 从 context 获取当前家庭ID
func GetFamilyID(c *gin.Context) uint64 {
	id, _ := c.Get("family_id")
	return id.(uint64)
}

// AuthRequired JWT 认证中间件
func AuthRequired() gin.HandlerFunc {
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
		c.Set("user_id", claims.UserID)
		c.Set("openid", claims.OpenID)
		c.Set("family_id", claims.FamilyID)
		c.Next()
	}
}
