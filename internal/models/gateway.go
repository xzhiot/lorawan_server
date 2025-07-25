package models

import (
    "time"
    
    "github.com/google/uuid"
)

// Gateway represents a LoRaWAN gateway
type Gateway struct {
    TenantModel
    
    GatewayID         EUI64      `json:"gatewayId" db:"gateway_id"`
    Name              string     `json:"name" db:"name"`
    Description       string     `json:"description" db:"description"`
    
    // Location
    Location          *Location  `json:"location,omitempty" db:"location"`
    
    // Configuration
    Model             string     `json:"model,omitempty" db:"model"`
    MinFrequency      uint32     `json:"minFrequency,omitempty" db:"min_frequency"`
    MaxFrequency      uint32     `json:"maxFrequency,omitempty" db:"max_frequency"`
    
    // Status
    LastSeenAt        *time.Time `json:"lastSeenAt,omitempty" db:"last_seen_at"`
    FirstSeenAt       *time.Time `json:"firstSeenAt,omitempty" db:"first_seen_at"`
    
    // Network settings
    NetworkServerID   *uuid.UUID `json:"networkServerId,omitempty" db:"network_server_id"`
    GatewayProfileID  *uuid.UUID `json:"gatewayProfileId,omitempty" db:"gateway_profile_id"`
    
    // Metadata
    Tags              Variables  `json:"tags,omitempty" db:"tags"`
    Metadata          Variables  `json:"metadata,omitempty" db:"metadata"`
}

// Location represents a geographic location
type Location struct {
    Latitude  float64 `json:"latitude" db:"latitude"`
    Longitude float64 `json:"longitude" db:"longitude"`
    Altitude  float64 `json:"altitude" db:"altitude"`
    Source    string  `json:"source,omitempty" db:"source"`
    Accuracy  int     `json:"accuracy,omitempty" db:"accuracy"`
}

// GatewayStats represents gateway statistics
type GatewayStats struct {
    ID                uuid.UUID  `json:"id" db:"id"`
    GatewayID         EUI64      `json:"gatewayId" db:"gateway_id"`
    Time              time.Time  `json:"time" db:"time"`
    
    // Packets
    RXPacketsReceived int        `json:"rxPacketsReceived" db:"rx_packets_received"`
    RXPacketsValid    int        `json:"rxPacketsValid" db:"rx_packets_valid"`
    TXPacketsEmitted  int        `json:"txPacketsEmitted" db:"tx_packets_emitted"`
    
    // Metadata
    Metadata          Variables  `json:"metadata,omitempty" db:"metadata"`
}

// GatewayProfile represents a gateway profile
type GatewayProfile struct {
    BaseModel
    NetworkServerID   *uuid.UUID `json:"networkServerId,omitempty" db:"network_server_id"`
    
    Name              string     `json:"name" db:"name"`
    Description       string     `json:"description" db:"description"`
    
    // Channels configuration
    Channels          Variables  `json:"channels" db:"channels"`
    
    // Extra channels
    ExtraChannels     Variables  `json:"extraChannels,omitempty" db:"extra_channels"`
}
