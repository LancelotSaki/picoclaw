package jwt

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Test_createToken(t *testing.T) {
	// 使用与 comp-agent 相同的 secret key
	secretKey := "ER%%GGR%R^%tge5t67y676t3e3davs"
	issuer := "cloud-portal" // 与 comp-agent 保持一致

	provider := NewJWTProvider(secretKey, issuer, 24*time.Hour)

	dto := TokenUserDTO{
		Account: "admin",
		Name:    "管理员",
	}
	token, err := provider.GenerateToken(&dto)
	if err != nil {
		fmt.Printf("生成token异常,%v", err)
		return
	}
	fmt.Printf("token=%s\n", token)

	// 验证 token (用 jwt 库直接验证)
	tokenObj, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		fmt.Printf("验证token异常,%v", err)
		return
	}
	if claims, ok := tokenObj.Claims.(*jwt.RegisteredClaims); ok && tokenObj.Valid {
		fmt.Printf("验证成功, subject=%s\n", claims.Subject)
	}
}

func Test_verifyWithCompAgent(t *testing.T) {
	// 这个测试模拟 comp-agent 验证 jwt-mcp 生成的 token
	secretKey := "ER%%GGR%R^%tge5t67y676t3e3davs"

	// 先用 jwt-mcp 的方式生成 token
	provider := NewJWTProvider(secretKey, "cloud-portal", 24*time.Hour)
	dto := &TokenUserDTO{
		Account: "admin",
		Name:    "管理员",
	}
	token, err := provider.GenerateToken(dto)
	if err != nil {
		t.Fatalf("生成token失败: %v", err)
	}
	fmt.Printf("生成的token: %s\n", token)

	// 模拟 comp-agent 的验证方式
	tokenObj, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		t.Fatalf("验证token失败: %v", err)
	}
	if !tokenObj.Valid {
		t.Fatal("token 无效")
	}
	fmt.Printf("comp-agent 验证成功!\n")
}
