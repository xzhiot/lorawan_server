package models

import (
    "database/sql/driver"
    "encoding/hex"
    "fmt"
    "time"
    
    "github.com/google/uuid"
)

// EUI64 represents an 8-byte Extended Unique Identifier
type EUI64 [8]byte

// String returns hex string representation
func (e EUI64) String() string {
    return hex.EncodeToString(e[:])
}

// MarshalJSON implements json.Marshaler
func (e EUI64) MarshalJSON() ([]byte, error) {
    return []byte(`"` + e.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (e *EUI64) UnmarshalJSON(data []byte) error {
    if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
        return fmt.Errorf("invalid EUI64 format")
    }
    
    b, err := hex.DecodeString(string(data[1 : len(data)-1]))
    if err != nil {
        return err
    }
    
    if len(b) != 8 {
        return fmt.Errorf("invalid EUI64 length")
    }
    
    copy(e[:], b)
    return nil
}

// Value implements driver.Valuer
func (e EUI64) Value() (driver.Value, error) {
    return e[:], nil
}

// Scan implements sql.Scanner
func (e *EUI64) Scan(value interface{}) error {
    if value == nil {
        return nil
    }
    
    switch v := value.(type) {
    case []byte:
        if len(v) != 8 {
            return fmt.Errorf("invalid EUI64 length")
        }
        copy(e[:], v)
        return nil
    default:
        return fmt.Errorf("cannot scan %T into EUI64", value)
    }
}

// DevAddr represents a 4-byte device address
type DevAddr [4]byte

// String returns hex string representation
func (d DevAddr) String() string {
    return hex.EncodeToString(d[:])
}

// Device represents a LoRaWAN device
type Device struct {
    TenantModel
    
    // Identifiers
    DevEUI          EUI64      `json:"devEUI" db:"dev_eui"`
    JoinEUI         *EUI64     `json:"joinEUI,omitempty" db:"join_eui"`
    DevAddr         *DevAddr   `json:"devAddr,omitempty" db:"dev_addr"`
    
    // Metadata
    Name            string     `json:"name" db:"name"`
    Description     string     `json:"description" db:"description"`
    ApplicationID   uuid.UUID  `json:"applicationId" db:"application_id"`
    DeviceProfileID uuid.UUID  `json:"deviceProfileId" db:"device_profile_id"`
    
    // Status
    IsDisabled      bool       `json:"isDisabled" db:"is_disabled"`
    LastSeenAt      *time.Time `json:"lastSeenAt,omitempty" db:"last_seen_at"`
    
    // Battery
    BatteryLevel          *float64   `json:"batteryLevel,omitempty" db:"battery_level"`
    BatteryLevelUpdatedAt *time.Time `json:"batteryLevelUpdatedAt,omitempty" db:"battery_level_updated_at"`
    
    // Session keys (for ABP)
    AppSKey      *string `json:"-" db:"app_s_key"`
    NwkSEncKey   *string `json:"-" db:"nwk_s_enc_key"`
    SNwkSIntKey  *string `json:"-" db:"s_nwk_s_int_key"`
    FNwkSIntKey  *string `json:"-" db:"f_nwk_s_int_key"`
    
    // Frame counters
    FCntUp      uint32 `json:"fCntUp" db:"f_cnt_up"`
    NFCntDown   uint32 `json:"nFCntDown" db:"n_f_cnt_down"`
    AFCntDown   uint32 `json:"aFCntDown" db:"a_f_cnt_down"`
    
    // Settings
    DR          *int `json:"dr,omitempty" db:"dr"`
    
    // Relations
    Application *Application `json:"application,omitempty"`
    Profile     *DeviceProfile `json:"profile,omitempty"`
}

// DeviceKeys represents device root keys (for OTAA)
type DeviceKeys struct {
    DevEUI   EUI64     `json:"devEUI" db:"dev_eui"`
    AppKey   string    `json:"appKey" db:"app_key"`
    NwkKey   string    `json:"nwkKey" db:"nwk_key"`
    UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

// DeviceActivation represents a device activation
type DeviceActivation struct {
    ID              uuid.UUID  `json:"id" db:"id"`
    DevEUI          EUI64      `json:"devEUI" db:"dev_eui"`
    DevAddr         DevAddr    `json:"devAddr" db:"dev_addr"`
    AppSKey         string     `json:"-" db:"app_s_key"`
    NwkSEncKey      string     `json:"-" db:"nwk_s_enc_key"`
    SNwkSIntKey     string     `json:"-" db:"s_nwk_s_int_key"`
    FNwkSIntKey     string     `json:"-" db:"f_nwk_s_int_key"`
    FCntUp          uint32     `json:"fCntUp" db:"f_cnt_up"`
    NFCntDown       uint32     `json:"nFCntDown" db:"n_f_cnt_down"`
    AFCntDown       uint32     `json:"aFCntDown" db:"a_f_cnt_down"`
    CreatedAt       time.Time  `json:"createdAt" db:"created_at"`
}

// DeviceProfile represents a device profile
type DeviceProfile struct {
    BaseModel
    TenantID             *uuid.UUID `json:"tenantId,omitempty" db:"tenant_id"`
    
    Name                 string     `json:"name" db:"name"`
    Description          string     `json:"description" db:"description"`
    
    // LoRaWAN
    MACVersion           string     `json:"macVersion" db:"mac_version"`
    RegParamsRevision    string     `json:"regParamsRevision" db:"reg_params_revision"`
    MaxEIRP              int        `json:"maxEIRP" db:"max_eirp"`
    MaxDutyCycle         int        `json:"maxDutyCycle" db:"max_duty_cycle"`
    RFRegion             string     `json:"rfRegion" db:"rf_region"`
    SupportsJoin         bool       `json:"supportsJoin" db:"supports_join"`
    Supports32BitFCnt    bool       `json:"supports32BitFCnt" db:"supports_32_bit_f_cnt"`
    
    // Class B
    SupportsClassB       bool       `json:"supportsClassB" db:"supports_class_b"`
    ClassBTimeout        int        `json:"classBTimeout" db:"class_b_timeout"`
    PingSlotPeriod       int        `json:"pingSlotPeriod" db:"ping_slot_period"`
    PingSlotDR           int        `json:"pingSlotDR" db:"ping_slot_dr"`
    PingSlotFreq         int        `json:"pingSlotFreq" db:"ping_slot_freq"`
    
    // Class C
    SupportsClassC       bool       `json:"supportsClassC" db:"supports_class_c"`
    ClassCTimeout        int        `json:"classCTimeout" db:"class_c_timeout"`
    
    // Uplink interval
    UplinkInterval       int        `json:"uplinkInterval" db:"uplink_interval"`
}


