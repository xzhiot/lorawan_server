package models

import (
    "time"
)

// DeviceSession represents an active device session
type DeviceSession struct {
    // Device identifiers
    DevEUI      EUI64
    DevAddr     DevAddr
    JoinEUI     EUI64
    
    // Session keys (stored as hex strings)
    FNwkSIntKey string
    SNwkSIntKey string
    NwkSEncKey  string
    AppSKey     string
    
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
    FCnt         uint32  `json:"fCnt"`
    MaxSNR       float64 `json:"maxSNR"`
    TXPower      uint8   `json:"txPower"`
    GatewayCount int     `json:"gatewayCount"`
}

// ActivationMode represents device activation mode
type ActivationMode string

const (
    ABP  ActivationMode = "ABP"
    OTAA ActivationMode = "OTAA"
)
