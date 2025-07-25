package auth

import (
    "fmt"
    "time"
    
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    
    "github.com/lorawan-server/lorawan-server-pro/internal/config"
    "github.com/lorawan-server/lorawan-server-pro/internal/models"
    "github.com/lorawan-server/lorawan-server-pro/pkg/crypto"
)

// JWTManager manages JWT tokens
type JWTManager struct {
    config *config.JWTConfig
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(cfg *config.JWTConfig) *JWTManager {
    return &JWTManager{
        config: cfg,
    }
}

// Claims represents JWT claims
type Claims struct {
    jwt.RegisteredClaims
    UserID   uuid.UUID  `json:"user_id"`
    Email    string     `json:"email"`
    IsAdmin  bool       `json:"is_admin"`
    TenantID *uuid.UUID `json:"tenant_id,omitempty"`
}

// GenerateTokenPair generates access and refresh tokens
func (m *JWTManager) GenerateTokenPair(user *models.User) (string, string, error) {
    // Access token
    accessClaims := Claims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   user.ID.String(),
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.config.AccessTokenTTL)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            NotBefore: jwt.NewNumericDate(time.Now()),
            Issuer:    "lorawan-server",
        },
        UserID:   user.ID,
        Email:    user.Email,
        IsAdmin:  user.IsAdmin,
        TenantID: user.TenantID,
    }
    
    accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
    accessTokenString, err := accessToken.SignedString([]byte(m.config.Secret))
    if err != nil {
        return "", "", fmt.Errorf("sign access token: %w", err)
    }
    
    // Refresh token
    refreshClaims := jwt.RegisteredClaims{
        Subject:   user.ID.String(),
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.config.RefreshTokenTTL)),
        IssuedAt:  jwt.NewNumericDate(time.Now()),
        NotBefore: jwt.NewNumericDate(time.Now()),
        Issuer:    "lorawan-server",
        ID:        uuid.New().String(),
    }
    
    refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
    refreshTokenString, err := refreshToken.SignedString([]byte(m.config.Secret))
    if err != nil {
        return "", "", fmt.Errorf("sign refresh token: %w", err)
    }
    
    return accessTokenString, refreshTokenString, nil
}

// ValidateToken validates a token
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(m.config.Secret), nil
    })
    
    if err != nil {
        return nil, err
    }
    
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }
    
    return claims, nil
}

// RefreshToken refreshes an access token
func (m *JWTManager) RefreshToken(refreshTokenString string) (string, string, error) {
    // Validate refresh token
    token, err := jwt.ParseWithClaims(refreshTokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(m.config.Secret), nil
    })
    
    if err != nil {
        return "", "", err
    }
    
    claims, ok := token.Claims.(*jwt.RegisteredClaims)
    if !ok || !token.Valid {
        return "", "", fmt.Errorf("invalid refresh token")
    }
    
    // TODO: Get user from database by subject
    userID, err := uuid.Parse(claims.Subject)
    if err != nil {
        return "", "", fmt.Errorf("invalid user ID in token")
    }
    
    // For now, create a minimal user object
    // In production, fetch from database
    user := &models.User{
        ID: userID,
    }
    
    return m.GenerateTokenPair(user)
}

// VerifyPassword verifies a password against a hash
func (m *JWTManager) VerifyPassword(password, hash string) bool {
    return crypto.VerifyPassword(password, hash)
}
