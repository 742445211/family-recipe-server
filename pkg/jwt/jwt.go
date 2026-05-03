// Package jwt 提供 JWT（JSON Web Token）的签发与解析功能。
// 使用 HS256 算法，Token 中包含用户 ID、OpenID 和当前家庭 ID。
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 载荷结构体，包含业务字段和 jwt 标准声明。
type Claims struct {
	UserID   uint64 `json:"user_id"`  // 用户 ID
	OpenID   string `json:"openid"`   // 微信 OpenID
	FamilyID uint64 `json:"family_id"` // 当前家庭 ID
	jwt.RegisteredClaims
}

// Generate 签发 JWT Token。
//
// 参数:
//   secret      - HMAC 签名密钥
//   expireHours - 过期小时数
//   userID      - 用户 ID
//   openID      - 微信 OpenID
//   familyID    - 当前家庭 ID
//
// 返回值:
//   string - 签发的 JWT 字符串
//   error  - 签发失败时返回错误
func Generate(secret string, expireHours int, userID uint64, openID string, familyID uint64) (string, error) {
	claims := Claims{
		UserID:   userID,
		OpenID:   openID,
		FamilyID: familyID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	// 使用 HS256 签名方法创建 Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Parse 解析并验证 JWT Token，返回载荷数据。
//
// 参数:
//   secret   - HMAC 签名密钥（须与签发时一致）
//   tokenStr - JWT 字符串
//
// 返回值:
//   *Claims - 解析出的载荷数据
//   error   - Token 无效或解析失败时返回错误
func Parse(secret, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		// 验证签名方法：仅接受 HMAC 算法
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	// 提取 Claims 并验证 Token 有效性
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
