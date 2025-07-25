package models

import (
    "time"
    
    "github.com/google/uuid"
)

// UplinkFrame represents an uplink frame
type UplinkFrame struct {
    ID            uuid.UUID    `json:"id" db:"id"`
    DevEUI        EUI64        `json:"devEUI" db:"dev_eui"`
    DevAddr       DevAddr      `json:"devAddr" db:"dev_addr"`
    ApplicationID uuid.UUID    `json:"applicationId" db:"application_id"`
    
    // PHY payload
    PHYPayload    []byte       `json:"phyPayload" db:"phy_payload"`
    
    // TX info
    TXInfo        interface{}  `json:"txInfo" db:"tx_info"`
    
    // RX info
    RXInfo        interface{}  `json:"rxInfo" db:"rx_info"`
    
    // Frame data
    FCnt          uint32       `json:"fCnt" db:"f_cnt"`
    FPort         *uint8       `json:"fPort,omitempty" db:"f_port"`
    DR            int          `json:"dr" db:"dr"`
    ADR           bool         `json:"adr" db:"adr"`
    
    // Decrypted data
    Data          []byte       `json:"data,omitempty" db:"data"`
    Object        interface{}  `json:"object,omitempty" db:"object"`
    
    // Confirmation
    Confirmed     bool         `json:"confirmed" db:"confirmed"`
    
    // Timestamp
    ReceivedAt    time.Time    `json:"receivedAt" db:"received_at"`
}

// DownlinkFrame represents a downlink frame
type DownlinkFrame struct {
    ID              uuid.UUID    `json:"id" db:"id"`
    DevEUI          EUI64        `json:"devEUI" db:"dev_eui"`
    ApplicationID   uuid.UUID    `json:"applicationId" db:"application_id"`
    
    // Frame data
    FPort           int          `json:"fPort" db:"f_port"`
    Data            []byte       `json:"data" db:"data"`
    Confirmed       bool         `json:"confirmed" db:"confirmed"`
    
    // State
    IsPending       bool         `json:"isPending" db:"is_pending"`
    RetryCount      int          `json:"retryCount" db:"retry_count"`
    
    // Timing
    CreatedAt       time.Time    `json:"createdAt" db:"created_at"`
    TransmittedAt   *time.Time   `json:"transmittedAt,omitempty" db:"transmitted_at"`
    AckedAt         *time.Time   `json:"acknowledgedAt,omitempty" db:"acked_at"`
    
    // Reference
    Reference       string       `json:"reference,omitempty" db:"reference"`
}

// RXInfo represents receive information
type RXInfo struct {
    GatewayID       EUI64        `json:"gatewayID"`
    RSSI            int          `json:"rssi"`
    LoRaSNR         float64      `json:"loRaSNR"`
    Location        *Location    `json:"location,omitempty"`
    UplinkID        string       `json:"uplinkID"`
    Time            *time.Time   `json:"time,omitempty"`
}

// GetRSSI returns the RSSI from RX info
func (f *UplinkFrame) GetRSSI() float64 {
    if f.RXInfo == nil {
        return 0
    }
    
    // Handle different RXInfo types
    switch rxInfo := f.RXInfo.(type) {
    case []interface{}:
        if len(rxInfo) > 0 {
            if m, ok := rxInfo[0].(map[string]interface{}); ok {
                if rssi, ok := m["rssi"].(float64); ok {
                    return rssi
                }
            }
        }
    case []RXInfo:
        if len(rxInfo) > 0 {
            return float64(rxInfo[0].RSSI)
        }
    case map[string]interface{}:
        if rssi, ok := rxInfo["rssi"].(float64); ok {
            return rssi
        }
    }
    
    return 0
}

// GetSNR returns the SNR from RX info
func (f *UplinkFrame) GetSNR() float64 {
    if f.RXInfo == nil {
        return 0
    }
    
    // Handle different RXInfo types
    switch rxInfo := f.RXInfo.(type) {
    case []interface{}:
        if len(rxInfo) > 0 {
            if m, ok := rxInfo[0].(map[string]interface{}); ok {
                if snr, ok := m["lsnr"].(float64); ok {
                    return snr
                }
            }
        }
    case []RXInfo:
        if len(rxInfo) > 0 {
            return rxInfo[0].LoRaSNR
        }
    case map[string]interface{}:
        if snr, ok := rxInfo["lsnr"].(float64); ok {
            return snr
        }
    }
    
    return 0
}
