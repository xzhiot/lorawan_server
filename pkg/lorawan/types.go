package lorawan

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// EUI64 represents an 8-byte Extended Unique Identifier
type EUI64 [8]byte

// String returns hex string representation
func (e EUI64) String() string {
	return hex.EncodeToString(e[:])
}

// MarshalJSON implements json.Marshaler
func (e EUI64) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (e *EUI64) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	if len(b) != 8 {
		return fmt.Errorf("invalid EUI64 length")
	}

	copy(e[:], b)
	return nil
}

// DevAddr represents a 4-byte device address
type DevAddr [4]byte

// String returns hex string representation
func (d DevAddr) String() string {
	return hex.EncodeToString(d[:])
}

// AES128Key represents a 128-bit AES key
type AES128Key [16]byte

// String returns hex string representation
func (k AES128Key) String() string {
	return hex.EncodeToString(k[:])
}

// MType represents the message type
type MType byte

const (
	JoinRequest MType = iota
	JoinAccept
	UnconfirmedDataUp
	UnconfirmedDataDown
	ConfirmedDataUp
	ConfirmedDataDown
	RFU
	Proprietary
)

// Major represents the LoRaWAN major version
type Major byte

const (
	LoRaWAN1_0 Major = 0
	LoRaWAN1_1 Major = 1
)

// PHYPayload represents the physical payload
type PHYPayload struct {
	MHDR       MHDR
	MACPayload []byte
	MIC        [4]byte
}

// MHDR represents the MAC header
type MHDR struct {
	MType MType
	Major Major
}

// MACPayload represents the MAC payload
type MACPayload struct {
	FHDR       FHDR
	FPort      *uint8
	FRMPayload []byte
}

// FHDR represents the frame header
type FHDR struct {
	DevAddr DevAddr
	FCtrl   FCtrl
	FCnt    uint16
	FOpts   []byte
}

// FCtrl represents the frame control byte
type FCtrl struct {
	ADR       bool
	ADRACKReq bool
	ACK       bool
	ClassB    bool
	FPending  bool
}

// JoinRequestPayload represents join request
type JoinRequestPayload struct {
	JoinEUI  EUI64
	DevEUI   EUI64
	DevNonce [2]byte
}

// JoinAcceptPayload represents join accept
type JoinAcceptPayload struct {
	JoinNonce  [3]byte
	NetID      [3]byte
	DevAddr    DevAddr
	DLSettings DLSettings
	RxDelay    uint8
	CFList     []byte
}

// DLSettings represents downlink settings
type DLSettings struct {
	RX1DROffset uint8
	RX2DataRate uint8
}
