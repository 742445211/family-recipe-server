package jwt

import (
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndParse(t *testing.T) {
	secret := "test-secret"
	token, err := Generate(secret, 1, 1, "test-openid", 1)
	if err != nil {
		t.Fatalf("生成token失败: %v", err)
	}
	if token == "" {
		t.Fatal("token为空")
	}

	claims, err := Parse(secret, token)
	if err != nil {
		t.Fatalf("解析token失败: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("UserID: want 1, got %d", claims.UserID)
	}
	if claims.OpenID != "test-openid" {
		t.Errorf("OpenID: want test-openid, got %s", claims.OpenID)
	}
}

func TestParseInvalidToken(t *testing.T) {
	_, err := Parse("secret", "invalid-token")
	if err == nil {
		t.Fatal("解析无效token应返回错误")
	}
}

func TestParseExpiredToken(t *testing.T) {
	secret := "test-secret"
	claims := Claims{
		UserID: 1,
		OpenID: "test",
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))

	_, err := Parse(secret, tokenStr)
	if err == nil {
		t.Fatal("过期token应返回错误")
	}
}

func TestParseWrongSecret(t *testing.T) {
	token, _ := Generate("secret-a", 1, 1, "test", 0)
	_, err := Parse("secret-b", token)
	if err == nil {
		t.Fatal("用错误secret解析应返回错误")
	}
}
