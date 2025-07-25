package models

import (
    "database/sql/driver"
    "time"
    
    "github.com/google/uuid"
)

// User represents a system user
type User struct {
    ID                  uuid.UUID   `json:"id" db:"id"`
    CreatedAt           time.Time   `json:"createdAt" db:"created_at"`
    UpdatedAt           time.Time   `json:"updatedAt" db:"updated_at"`
    
    Email               string      `json:"email" db:"email"`
    Username            string      `json:"username" db:"username"`
    FirstName           string      `json:"firstName" db:"first_name"`
    LastName            string      `json:"lastName" db:"last_name"`
    
    PasswordHash        string      `json:"-" db:"password_hash"`
    
    IsAdmin             bool        `json:"isAdmin" db:"is_admin"`
    IsActive            bool        `json:"isActive" db:"is_active"`
    
    EmailVerified       bool        `json:"emailVerified" db:"email_verified"`
    EmailVerifiedAt     *time.Time  `json:"emailVerifiedAt,omitempty" db:"email_verified_at"`
    
    LastLoginAt         *time.Time  `json:"lastLoginAt,omitempty" db:"last_login_at"`
    
    TenantID            *uuid.UUID  `json:"tenantId,omitempty" db:"tenant_id"`
    
    Settings            Variables   `json:"settings" db:"settings"`
}

// APIKey represents an API key
type APIKey struct {
    ID                  uuid.UUID   `json:"id" db:"id"`
    CreatedAt           time.Time   `json:"createdAt" db:"created_at"`
    
    UserID              uuid.UUID   `json:"userId" db:"user_id"`
    TenantID            *uuid.UUID  `json:"tenantId,omitempty" db:"tenant_id"`
    
    Name                string      `json:"name" db:"name"`
    Key                 string      `json:"key" db:"key"`
    
    IsActive            bool        `json:"isActive" db:"is_active"`
    ExpiresAt           *time.Time  `json:"expiresAt,omitempty" db:"expires_at"`
    LastUsedAt          *time.Time  `json:"lastUsedAt,omitempty" db:"last_used_at"`
    
    Scopes              StringArray `json:"scopes" db:"scopes"`
}

// StringArray represents a PostgreSQL string array
type StringArray []string

// Value implements driver.Valuer
func (a StringArray) Value() (driver.Value, error) {
    if a == nil {
        return nil, nil
    }
    return a, nil
}

// Scan implements sql.Scanner
func (a *StringArray) Scan(value interface{}) error {
    if value == nil {
        *a = nil
        return nil
    }
    
    switch v := value.(type) {
    case []string:
        *a = v
        return nil
    default:
        return nil
    }
}
