package jwt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenUserDTO 对应Java版本的TokenUserDTO
type TokenUserDTO struct {
	ID         string   `json:"id"`
	Account    string   `json:"account"`     // 账号（必填）
	Name       string   `json:"name"`        // 姓名
	EmployeeID string   `json:"employeeId"`  // 员工ID
	Mobile     string   `json:"mobile"`      // 手机号
	Email      string   `json:"email"`       // 邮箱
	Status     string   `json:"status"`      // 状态
	OrgID      string   `json:"orgId"`       // 组织ID
	Code       string   `json:"code"`        // 编码
	Tenants    []string `json:"tenants"`     // 租户列表
	SysRoles   []string `json:"sysRoles"`    // 系统角色
	SysRoleIDs []string `json:"sysRoleIds"`  // 系统角色ID列表
	EntID      string   `json:"entId"`       // 企业编号
	TenantID   string   `json:"tenantId"`    // 租户ID
	Type       string   `json:"type"`        // 类型
	LoginIP    string   `json:"loginIp"`     // 登录IP
}

// ToJSON 将用户信息转为JSON字符串（用于JWT的subject）
func (u *TokenUserDTO) ToJSON() (string, error) {
	data, err := json.Marshal(u)
	return string(data), err
}

// FromJSON 从JSON字符串解析用户信息
func FromJSON(jsonStr string) (*TokenUserDTO, error) {
	var user TokenUserDTO
	err := json.Unmarshal([]byte(jsonStr), &user)
	return &user, err
}

// JWTProvider JWT Provider
type JWTProvider struct {
	secretKey  []byte
	issuer     string
	expireTime time.Duration
}

// NewJWTProvider 创建JWT Provider
func NewJWTProvider(secretKey, issuer string, expireTime time.Duration) *JWTProvider {
	return &JWTProvider{
		secretKey:  []byte(secretKey),
		issuer:     issuer,
		expireTime: expireTime,
	}
}

// Connect connects with the given configuration (no-op for JWT)
func (p *JWTProvider) Connect(ctx context.Context, config *JWTConfig) error {
	if config == nil {
		return fmt.Errorf("JWT config is required")
	}

	if config.SecretKey == "" {
		return fmt.Errorf("JWT secret key is required")
	}

	p.secretKey = []byte(config.SecretKey)
	p.issuer = config.Issuer
	if config.ExpireTime > 0 {
		p.expireTime = time.Duration(config.ExpireTime) * time.Hour
	} else {
		p.expireTime = 24 * time.Hour // default 24 hours
	}

	return nil
}

// Close closes the provider (no-op for JWT)
func (p *JWTProvider) Close() error {
	return nil
}

// IsConnected returns true if the provider is configured
func (p *JWTProvider) IsConnected() bool {
	return len(p.secretKey) > 0 && p.issuer != ""
}

// GenerateToken 生成JWT Token
func (p *JWTProvider) GenerateToken(user *TokenUserDTO) (string, error) {
	// 将用户信息转为JSON作为subject
	subject, err := user.ToJSON()
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		ID:        generateJWTID(),                      // JWT ID
		Issuer:    p.issuer,                             // 签发者
		Subject:   subject,                              // 用户信息（JSON字符串）
		IssuedAt:  jwt.NewNumericDate(now),              // 签发时间
		NotBefore: jwt.NewNumericDate(now),              // 生效时间
		ExpiresAt: jwt.NewNumericDate(now.Add(p.expireTime)), // 过期时间
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.secretKey)
}

// Execute 执行JWT工具
func (p *JWTProvider) Execute(ctx context.Context, req *GenerateTokenRequest) (*GenerateTokenResult, error) {
	if !p.IsConnected() {
		return nil, fmt.Errorf("JWT provider is not connected")
	}

	user := &TokenUserDTO{
		Account: req.Account,
		Name:    req.Username,
	}

	token, err := p.GenerateToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &GenerateTokenResult{
		Token:     token,
		ExpiresAt: time.Now().Add(p.expireTime).Format(time.RFC3339),
	}, nil
}

// generateJWTID 生成JWT ID
func generateJWTID() string {
	return uuid.New().String()
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	// SecretKey is the secret key for signing JWT tokens
	SecretKey string `json:"secret_key"`
	// Issuer is the JWT issuer
	Issuer string `json:"issuer"`
	// ExpireTime is the token expiration time in hours
	ExpireTime int `json:"expire_time"`
}

// GenerateTokenRequest represents a request to generate a JWT token
type GenerateTokenRequest struct {
	// Account is the user account (required)
	Account string `json:"account"`
	// Username is the user name (optional, defaults to "admin")
	Username string `json:"username"`
}

// GenerateTokenResult represents the result of generating a JWT token
type GenerateTokenResult struct {
	// Token is the generated JWT token
	Token string `json:"token"`
	// ExpiresAt is the token expiration time
	ExpiresAt string `json:"expires_at"`
}

// ServerConfig holds command line configuration
type ServerConfig struct {
	SecretKey  string `json:"secret_key"`
	Issuer     string `json:"issuer"`
	ExpireTime int    `json:"expire_time"`
}

// JWTProviderFactory is a function that creates a new JWTProvider
type JWTProviderFactory func() *JWTProvider

// Default secret key (same as comp-agent)
const DefaultSecretKey = "ER%%GGR%R^%tge5t67y676t3e3davs"
const DefaultIssuer = "cloud-portal"

// NewDefaultJWTProvider creates a default JWT provider
func NewDefaultJWTProvider() *JWTProvider {
	return NewJWTProvider(DefaultSecretKey, DefaultIssuer, 24*time.Hour)
}

// ConnectProvider creates a provider and connects to the JWT service
func ConnectProvider(ctx context.Context, config *JWTConfig) (*JWTProvider, error) {
	provider := NewDefaultJWTProvider()
	if err := provider.Connect(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to connect to JWT service: %w", err)
	}
	return provider, nil
}

// JWTProviderInterface defines the interface for JWT server operations
type JWTProviderInterface interface {
	Execute(ctx context.Context, req *GenerateTokenRequest) (*GenerateTokenResult, error)
	Close() error
	IsConnected() bool
}

var _ JWTProviderInterface = (*JWTProvider)(nil)
