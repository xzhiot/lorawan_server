package models

import (
    "time"
    
    "github.com/google/uuid"
)

// Tenant represents a tenant/organization
type Tenant struct {
    ID               uuid.UUID   `json:"id" db:"id"`
    CreatedAt        time.Time   `json:"createdAt" db:"created_at"`
    UpdatedAt        time.Time   `json:"updatedAt" db:"updated_at"`
    
    Name             string      `json:"name" db:"name"`
    Description      string      `json:"description" db:"description"`
    
    // Limits
    MaxGatewayCount  int         `json:"maxGatewayCount" db:"max_gateway_count"`
    MaxDeviceCount   int         `json:"maxDeviceCount" db:"max_device_count"`
    MaxUserCount     int         `json:"maxUserCount" db:"max_user_count"`
    
    // Features
    CanHaveGateways  bool        `json:"canHaveGateways" db:"can_have_gateways"`
    PrivateGateways  bool        `json:"privateGateways" db:"private_gateways"`
    
    // Billing
    BillingEmail     string      `json:"billingEmail,omitempty" db:"billing_email"`
    BillingPlan      string      `json:"billingPlan,omitempty" db:"billing_plan"`
    
    // Status
    IsActive         bool        `json:"isActive" db:"is_active"`
    SuspendedAt      *time.Time  `json:"suspendedAt,omitempty" db:"suspended_at"`
}

// TenantUser represents a user-tenant association
type TenantUser struct {
    UserID           uuid.UUID   `json:"userId" db:"user_id"`
    TenantID         uuid.UUID   `json:"tenantId" db:"tenant_id"`
    IsAdmin          bool        `json:"isAdmin" db:"is_admin"`
    IsDeviceAdmin    bool        `json:"isDeviceAdmin" db:"is_device_admin"`
    IsGatewayAdmin   bool        `json:"isGatewayAdmin" db:"is_gateway_admin"`
    CreatedAt        time.Time   `json:"createdAt" db:"created_at"`
    UpdatedAt        time.Time   `json:"updatedAt" db:"updated_at"`
}
