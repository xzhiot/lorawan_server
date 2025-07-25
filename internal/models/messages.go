package models

import (
    "time"
)

// UplinkMessage represents an uplink message from gateway
type UplinkMessage struct {
    PHYPayloadBytes []byte
    PHYPayload      interface{}
    TXInfo          interface{}
    RXInfo          []RXInfo
    GatewayMAC      EUI64
    ReceivedAt      time.Time
    Frequency       uint32
    DR              int
    RSSI            float64
    SNR             float64
}

// DownlinkMessage represents a downlink message to gateway
type DownlinkMessage struct {
    DevEUI      EUI64
    FPort       uint8
    Data        []byte
    Confirmed   bool
    FCnt        uint32
    GatewayMAC  EUI64
    Frequency   uint32
    Power       int
    DataRate    int
    Delay       time.Duration
    ScheduledAt time.Time
}

// JoinRequestMessage represents a join request
type JoinRequestMessage struct {
    PHYPayload []byte
    DevEUI     EUI64
    JoinEUI    EUI64
    DevNonce   [2]byte
    RXInfo     []RXInfo
    GatewayMAC EUI64
}

// JoinAcceptMessage represents a join accept
type JoinAcceptMessage struct {
    PHYPayload []byte
    DevEUI     EUI64
    DevAddr    DevAddr
    JoinNonce  [3]byte
}
