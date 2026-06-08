// Package middleware 提供 Gin 中间件。
// 包含 JWT 认证中间件，用于从 Authorization header 中解析 Token，
// 并将用户 ID、OpenID、家庭 ID 注入请求上下文供后续 handler 使用。
package middleware

import (
	"net/http"
	"strings"

	"recipe-server/config"
	"recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
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

// AuthRequired 返回 JWT 认证中间件。
// 从请求头 Authorization: Bearer <token> 中提取 JWT，
// 校验通过后将 user_id、openid、family_id 写入 context，
// 校验失败返回 401。
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取 Authorization 请求头
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
			c.Abort()
			return
		}

		// 2. 校验 Bearer 格式
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "认证格式错误"})
			c.Abort()
			return
		}

		// 3. 解析并验证 JWT
		claims, err := jwt.Parse(config.AppConfig.JWT.Secret, parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Token无效或已过期"})
			c.Abort()
			return
		}

		// 4. 将用户信息注入上下文，供后续 handler 使用
		c.Set("user_id", claims.UserID)
		c.Set("openid", claims.OpenID)
		c.Set("family_id", claims.FamilyID)
		c.Next()
	}
}

// OptionalAuth 可选 JWT 认证：有合法 Bearer token 时注入用户信息，否则继续处理。
func OptionalAuth() gin.HandlerFunc {
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
		c.Set("user_id", claims.UserID)
		c.Set("openid", claims.OpenID)
		c.Set("family_id", claims.FamilyID)
		c.Next()
	}
}
