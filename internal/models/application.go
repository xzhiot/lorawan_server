package models

import (
	"github.com/google/uuid"
)

// Application represents an application
type Application struct {
	TenantModel

	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`

	// Integration settings
	HTTPIntegration *Variables `json:"httpIntegration,omitempty" db:"http_integration"`
	MQTTIntegration *Variables `json:"mqttIntegration,omitempty" db:"mqtt_integration"`

	// Decoder settings
	PayloadCodec   string `json:"payloadCodec" db:"payload_codec"`
	PayloadDecoder string `json:"payloadDecoder,omitempty" db:"payload_decoder"`
	PayloadEncoder string `json:"payloadEncoder,omitempty" db:"payload_encoder"`

	// Statistics
	DeviceCount int `json:"deviceCount,omitempty"`
}

// Integration represents an application integration
type Integration struct {
	BaseModel

	ApplicationID uuid.UUID `json:"applicationId" db:"application_id"`
	Type          string    `json:"type" db:"type"`
	Settings      Variables `json:"settings" db:"settings"`
	IsEnabled     bool      `json:"isEnabled" db:"is_enabled"`
}

// IntegrationType represents integration types
type IntegrationType string

const (
	IntegrationHTTP     IntegrationType = "HTTP"
	IntegrationMQTT     IntegrationType = "MQTT"
	IntegrationInfluxDB IntegrationType = "InfluxDB"
	IntegrationAWS      IntegrationType = "AWS"
)
