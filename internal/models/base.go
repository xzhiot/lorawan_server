package models

import (
    "database/sql/driver"
    "encoding/json"
    "time"
    
    "github.com/google/uuid"
)

// BaseModel contains common fields for all models
type BaseModel struct {
    ID        uuid.UUID  `json:"id" db:"id"`
    CreatedAt time.Time  `json:"createdAt" db:"created_at"`
    UpdatedAt time.Time  `json:"updatedAt" db:"updated_at"`
}

// TenantModel extends BaseModel with tenant support
type TenantModel struct {
    BaseModel
    TenantID uuid.UUID `json:"tenantId" db:"tenant_id"`
}

// Variables represents a JSON object for storing arbitrary data
type Variables map[string]interface{}

// Value implements driver.Valuer interface
func (v Variables) Value() (driver.Value, error) {
    if v == nil {
        return nil, nil
    }
    return json.Marshal(v)
}

// Scan implements sql.Scanner interface
func (v *Variables) Scan(value interface{}) error {
    if value == nil {
        *v = make(Variables)
        return nil
    }
    
    switch data := value.(type) {
    case []byte:
        return json.Unmarshal(data, v)
    case string:
        return json.Unmarshal([]byte(data), v)
    default:
        return json.Unmarshal([]byte(data.(string)), v)
    }
}
