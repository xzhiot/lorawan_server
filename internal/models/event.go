package models

import (
    "time"
    
    "github.com/google/uuid"
)

// EventLog represents an event log entry
type EventLog struct {
    ID               uuid.UUID   `json:"id" db:"id"`
    CreatedAt        time.Time   `json:"createdAt" db:"created_at"`
    
    TenantID         *uuid.UUID  `json:"tenantId,omitempty" db:"tenant_id"`
    ApplicationID    *uuid.UUID  `json:"applicationId,omitempty" db:"application_id"`
    DevEUI           *EUI64      `json:"devEUI,omitempty" db:"dev_eui"`
    GatewayID        *EUI64      `json:"gatewayId,omitempty" db:"gateway_id"`
    
    Type             EventType   `json:"type" db:"type"`
    Level            EventLevel  `json:"level" db:"level"`
    Code             string      `json:"code" db:"code"`
    Description      string      `json:"description" db:"description"`
    
    Details          Variables   `json:"details,omitempty" db:"details"`
}

// EventType represents event types
type EventType string

const (
    // Device events
    EventTypeUplink         EventType = "UPLINK"
    EventTypeDownlink       EventType = "DOWNLINK"
    EventTypeJoin           EventType = "JOIN"
    EventTypeAck            EventType = "ACK"
    EventTypeError          EventType = "ERROR"
    
    // Gateway events
    EventTypeGatewayUp      EventType = "GATEWAY_UP"
    EventTypeGatewayDown    EventType = "GATEWAY_DOWN"
    EventTypeGatewayStats   EventType = "GATEWAY_STATS"
    
    // System events
    EventTypeAPICall        EventType = "API_CALL"
    EventTypeIntegration    EventType = "INTEGRATION"
    EventTypeDownlinkQueued EventType = "DOWNLINK_QUEUED"
    EventTypeDownlinkAck    EventType = "DOWNLINK_ACK"
)

// EventLevel represents event severity levels
type EventLevel string

const (
    EventLevelDebug   EventLevel = "DEBUG"
    EventLevelInfo    EventLevel = "INFO"
    EventLevelWarning EventLevel = "WARNING"
    EventLevelError   EventLevel = "ERROR"
    EventLevelFatal   EventLevel = "FATAL"
)
