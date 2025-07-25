package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage"
	"github.com/lorawan-server/lorawan-server-pro/pkg/lorawan"
)

// NATSSubscriber NATS subscriber
type NATSSubscriber struct {
	nc    *nats.Conn
	store storage.Store
	subs  []*nats.Subscription
}

// NewNATSSubscriber creates NATS subscriber
func NewNATSSubscriber(nc *nats.Conn, store storage.Store) *NATSSubscriber {
	return &NATSSubscriber{
		nc:    nc,
		store: store,
		subs:  make([]*nats.Subscription, 0),
	}
}

// Start starts subscriptions
func (s *NATSSubscriber) Start(ctx context.Context) error {
	// Subscribe to application data from network server
	sub1, err := s.nc.Subscribe("application.*.device.*.rx", s.handleApplicationUplink)
	if err != nil {
		return fmt.Errorf("subscribe application uplink: %w", err)
	}
	s.subs = append(s.subs, sub1)

	// Subscribe to join notifications
	sub2, err := s.nc.Subscribe("application.*.device.*.join", s.handleJoinNotification)
	if err != nil {
		return fmt.Errorf("subscribe join notification: %w", err)
	}
	s.subs = append(s.subs, sub2)

	// Subscribe to device status
	sub3, err := s.nc.Subscribe("application.*.device.*.status", s.handleDeviceStatus)
	if err != nil {
		return fmt.Errorf("subscribe device status: %w", err)
	}
	s.subs = append(s.subs, sub3)

	// Subscribe to downlink acknowledgments
	sub4, err := s.nc.Subscribe("application.*.device.*.ack", s.handleDownlinkAck)
	if err != nil {
		return fmt.Errorf("subscribe downlink ack: %w", err)
	}
	s.subs = append(s.subs, sub4)

	log.Info().
		Int("subscriptions", len(s.subs)).
		Msg("NATS subscriber started")

	<-ctx.Done()

	// Unsubscribe
	for _, sub := range s.subs {
		sub.Unsubscribe()
	}

	return ctx.Err()
}

// handleApplicationUplink handles application uplink messages
func (s *NATSSubscriber) handleApplicationUplink(msg *nats.Msg) {
	log.Debug().
		Str("subject", msg.Subject).
		Int("size", len(msg.Data)).
		Msg("Received application uplink")

	var uplinkMsg struct {
		ApplicationID string                   `json:"applicationID"`
		DevEUI        string                   `json:"devEUI"`
		DeviceInfo    map[string]interface{}   `json:"deviceInfo"`
		FCnt          uint32                   `json:"fCnt"`
		FPort         uint8                    `json:"fPort"`
		Data          []byte                   `json:"data"`
		Object        interface{}              `json:"object,omitempty"`
		RXInfo        []map[string]interface{} `json:"rxInfo"`
		TXInfo        map[string]interface{}   `json:"txInfo"`
	}

	if err := json.Unmarshal(msg.Data, &uplinkMsg); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal application uplink")
		return
	}

	// Log event
	ctx := context.Background()

	appID, _ := uuid.Parse(uplinkMsg.ApplicationID)
	devEUI, _ := hex.DecodeString(uplinkMsg.DevEUI)

	event := &models.EventLog{
		ApplicationID: &appID,
		DevEUI:        (*models.EUI64)(devEUI),
		Type:          models.EventTypeUplink,
		Level:         models.EventLevelInfo,
		Description:   fmt.Sprintf("Application data received - FCnt: %d, FPort: %d", uplinkMsg.FCnt, uplinkMsg.FPort),
		Details: models.Variables{
			"fCnt":     uplinkMsg.FCnt,
			"fPort":    uplinkMsg.FPort,
			"dataSize": len(uplinkMsg.Data),
		},
	}

	if err := s.store.CreateEventLog(ctx, event); err != nil {
		log.Error().Err(err).Msg("Failed to create event log")
	}

	log.Info().
		Str("devEUI", uplinkMsg.DevEUI).
		Uint32("fCnt", uplinkMsg.FCnt).
		Uint8("fPort", uplinkMsg.FPort).
		Int("dataLen", len(uplinkMsg.Data)).
		Msg("Application uplink processed")
}

// handleJoinNotification handles join notifications
func (s *NATSSubscriber) handleJoinNotification(msg *nats.Msg) {
	log.Debug().
		Str("subject", msg.Subject).
		Msg("Received join notification")

	var joinMsg struct {
		ApplicationID string `json:"applicationID"`
		DevEUI        string `json:"devEUI"`
		DevAddr       string `json:"devAddr"`
	}

	if err := json.Unmarshal(msg.Data, &joinMsg); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal join notification")
		return
	}

	// Log join event
	ctx := context.Background()
	appID, _ := uuid.Parse(joinMsg.ApplicationID)
	devEUI, _ := hex.DecodeString(joinMsg.DevEUI)

	event := &models.EventLog{
		ApplicationID: &appID,
		DevEUI:        (*models.EUI64)(devEUI),
		Type:          models.EventTypeJoin,
		Level:         models.EventLevelInfo,
		Description:   "Device joined network",
		Details: models.Variables{
			"devAddr": joinMsg.DevAddr,
		},
	}

	if err := s.store.CreateEventLog(ctx, event); err != nil {
		log.Error().Err(err).Msg("Failed to create event log")
	}

	log.Info().
		Str("devEUI", joinMsg.DevEUI).
		Str("devAddr", joinMsg.DevAddr).
		Msg("Device joined")
}

// handleDeviceStatus handles device status updates
func (s *NATSSubscriber) handleDeviceStatus(msg *nats.Msg) {
	log.Debug().
		Str("subject", msg.Subject).
		Msg("Received device status")

	var statusMsg struct {
		ApplicationID string `json:"applicationID"`
		DevEUI        string `json:"devEUI"`
		Battery       uint8  `json:"battery"`
		Margin        int8   `json:"margin"`
		LastSeenAt    string `json:"lastSeenAt"`
	}

	if err := json.Unmarshal(msg.Data, &statusMsg); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal device status")
		return
	}

	// Update device status
	ctx := context.Background()
	devEUI, _ := hex.DecodeString(statusMsg.DevEUI)
	device, err := s.store.GetDevice(ctx, lorawan.EUI64(devEUI))
	if err != nil {
		log.Error().Err(err).Msg("Failed to get device")
		return
	}

	// Update battery level
	if statusMsg.Battery != 255 {
		batteryLevel := float64(statusMsg.Battery)
		device.BatteryLevel = &batteryLevel
		now := time.Now()
		device.BatteryLevelUpdatedAt = &now
	}

	// Update last seen
	if lastSeen, err := time.Parse(time.RFC3339, statusMsg.LastSeenAt); err == nil {
		device.LastSeenAt = &lastSeen
	}

	if err := s.store.UpdateDevice(ctx, device); err != nil {
		log.Error().Err(err).Msg("Failed to update device")
	}

	log.Info().
		Str("devEUI", statusMsg.DevEUI).
		Uint8("battery", statusMsg.Battery).
		Int8("margin", statusMsg.Margin).
		Msg("Device status updated")
}

// handleDownlinkAck handles downlink acknowledgments
func (s *NATSSubscriber) handleDownlinkAck(msg *nats.Msg) {
	log.Debug().
		Str("subject", msg.Subject).
		Msg("Received downlink ack")

	var ackMsg struct {
		ApplicationID string `json:"applicationID"`
		DevEUI        string `json:"devEUI"`
		FCnt          uint32 `json:"fCnt"`
		Acknowledged  bool   `json:"acknowledged"`
	}

	if err := json.Unmarshal(msg.Data, &ackMsg); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal downlink ack")
		return
	}

	// Log acknowledgment
	ctx := context.Background()
	appID, _ := uuid.Parse(ackMsg.ApplicationID)
	devEUI, _ := hex.DecodeString(ackMsg.DevEUI)

	event := &models.EventLog{
		ApplicationID: &appID,
		DevEUI:        (*models.EUI64)(devEUI),
		Type:          models.EventTypeDownlinkAck,
		Level:         models.EventLevelInfo,
		Description: fmt.Sprintf("Downlink %s - FCnt: %d",
			map[bool]string{true: "acknowledged", false: "not acknowledged"}[ackMsg.Acknowledged],
			ackMsg.FCnt),
		Details: models.Variables{
			"fCnt":         ackMsg.FCnt,
			"acknowledged": ackMsg.Acknowledged,
		},
	}

	if err := s.store.CreateEventLog(ctx, event); err != nil {
		log.Error().Err(err).Msg("Failed to create event log")
	}

	log.Info().
		Str("devEUI", ackMsg.DevEUI).
		Uint32("fCnt", ackMsg.FCnt).
		Bool("ack", ackMsg.Acknowledged).
		Msg("Downlink acknowledgment received")
}

// handleApplicationDownlink handles application downlink requests
func (s *NATSSubscriber) handleApplicationDownlink(msg *nats.Msg) {
	log.Debug().
		Str("subject", msg.Subject).
		Msg("Received application downlink request")

	var downReq struct {
		DevEUI    string `json:"devEUI"`
		FPort     uint8  `json:"fPort"`
		Data      []byte `json:"data"`
		Confirmed bool   `json:"confirmed"`
		Reference string `json:"reference"`
	}

	if err := json.Unmarshal(msg.Data, &downReq); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal downlink request")
		return
	}

	// Forward to network server
	ctx := context.Background()
	devEUI, err := hex.DecodeString(downReq.DevEUI)
	if err != nil {
		log.Error().Err(err).Msg("Invalid DevEUI")
		return
	}

	// Get device info
	device, err := s.store.GetDevice(ctx, lorawan.EUI64(devEUI))
	if err != nil {
		log.Error().Err(err).Msg("Failed to get device")
		return
	}

	// Create downlink frame record
	frame := &models.DownlinkFrame{
		DevEUI:        device.DevEUI,
		ApplicationID: device.ApplicationID,
		FPort:         int(downReq.FPort),
		Data:          downReq.Data,
		Confirmed:     downReq.Confirmed,
		Reference:     downReq.Reference,
	}

	if err := s.store.CreateDownlinkFrame(ctx, frame); err != nil {
		log.Error().Err(err).Msg("Failed to create downlink frame")
		return
	}

	// Publish to network server
	nsMsg := map[string]interface{}{
		"devEUI":    downReq.DevEUI,
		"fPort":     downReq.FPort,
		"data":      downReq.Data,
		"confirmed": downReq.Confirmed,
		"id":        frame.ID.String(),
	}

	data, _ := json.Marshal(nsMsg)
	subject := fmt.Sprintf("ns.device.%s.tx", downReq.DevEUI)

	if err := s.nc.Publish(subject, data); err != nil {
		log.Error().Err(err).Msg("Failed to publish downlink to NS")
		return
	}

	log.Info().
		Str("devEUI", downReq.DevEUI).
		Uint8("fPort", downReq.FPort).
		Int("dataLen", len(downReq.Data)).
		Msg("Downlink request forwarded to NS")
}
