package lorawan

import (
    "time"
)

// DeviceSession represents an active device session
type DeviceSession struct {
    // Device identifiers
    DevEUI      EUI64
    DevAddr     DevAddr
    JoinEUI     EUI64
    
    // Session keys
    FNwkSIntKey AES128Key
    SNwkSIntKey AES128Key
    NwkSEncKey  AES128Key
    AppSKey     AES128Key
    
    // Frame counters
    FCntUp      uint32
    FCntDown    uint32
    NFCntDown   uint32
    AFCntDown   uint32
    ConfFCnt    uint32
    
    // RX windows
    RX1Delay       uint8
    RX1DROffset    uint8
    RX2DR          uint8
    RX2Freq        uint32
    RXDelay        uint8
    
    // Device settings
    DR             uint8
    TXPower        uint8
    NbTrans        uint8
    EnabledChannels []int
    
    // ADR
    ADR            bool
    ADRHistory     []ADRHistory
    
    // Timestamps
    LastDevStatusRequest time.Time
    CreatedAt           time.Time
    UpdatedAt           time.Time
}

// ADRHistory represents ADR history entry
type ADRHistory struct {
    FCnt        uint32
    MaxSNR      float64
    TXPower     uint8
    GatewayCount int
}

// ActivationMode represents device activation mode
type ActivationMode string

const (
    ABP  ActivationMode = "ABP"
    OTAA ActivationMode = "OTAA"
)
