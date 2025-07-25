package lorawan

import (
    "fmt"
)

// MACCommand represents a MAC command
type MACCommand struct {
    CID     byte
    Payload []byte
}

// MAC command identifiers
const (
    LinkCheckReq     byte = 0x02
    LinkCheckAns     byte = 0x02
    LinkADRReq       byte = 0x03
    LinkADRAns       byte = 0x03
    DutyCycleReq     byte = 0x04
    DutyCycleAns     byte = 0x04
    RXParamSetupReq  byte = 0x05
    RXParamSetupAns  byte = 0x05
    DevStatusReq     byte = 0x06
    DevStatusAns     byte = 0x06
    NewChannelReq    byte = 0x07
    NewChannelAns    byte = 0x07
    RXTimingSetupReq byte = 0x08
    RXTimingSetupAns byte = 0x08
    TxParamSetupReq  byte = 0x09
    TxParamSetupAns  byte = 0x09
    DlChannelReq     byte = 0x0A
    DlChannelAns     byte = 0x0A
    DeviceTimeReq    byte = 0x0D
    DeviceTimeAns    byte = 0x0D
)

// ParseMACCommands parses MAC commands from bytes
func ParseMACCommands(uplink bool, data []byte) ([]MACCommand, error) {
    var commands []MACCommand
    
    for i := 0; i < len(data); {
        if i >= len(data) {
            break
        }
        
        cmd := MACCommand{
            CID: data[i],
        }
        i++
        
        payloadLen := getMACCommandPayloadLength(uplink, cmd.CID)
        if payloadLen < 0 {
            return nil, fmt.Errorf("unknown MAC command: %02x", cmd.CID)
        }
        
        if i+payloadLen > len(data) {
            return nil, fmt.Errorf("insufficient data for MAC command")
        }
        
        cmd.Payload = data[i : i+payloadLen]
        i += payloadLen
        
        commands = append(commands, cmd)
    }
    
    return commands, nil
}

// getMACCommandPayloadLength returns the payload length for a MAC command
func getMACCommandPayloadLength(uplink bool, cid byte) int {
    if uplink {
        switch cid {
        case LinkCheckReq:
            return 0
        case LinkADRAns:
            return 1
        case DutyCycleAns:
            return 0
        case RXParamSetupAns:
            return 1
        case DevStatusAns:
            return 2
        case NewChannelAns:
            return 1
        case RXTimingSetupAns:
            return 0
        case TxParamSetupAns:
            return 0
        case DlChannelAns:
            return 1
        case DeviceTimeReq:
            return 0
        default:
            return -1
        }
    } else {
        switch cid {
        case LinkCheckAns:
            return 2
        case LinkADRReq:
            return 4
        case DutyCycleReq:
            return 1
        case RXParamSetupReq:
            return 4
        case DevStatusReq:
            return 0
        case NewChannelReq:
            return 5
        case RXTimingSetupReq:
            return 1
        case TxParamSetupReq:
            return 1
        case DlChannelReq:
            return 4
        case DeviceTimeAns:
            return 5
        default:
            return -1
        }
    }
}

// EncodeMACCommands encodes MAC commands to bytes
func EncodeMACCommands(commands []MACCommand) ([]byte, error) {
    var data []byte
    
    for _, cmd := range commands {
        data = append(data, cmd.CID)
        data = append(data, cmd.Payload...)
    }
    
    return data, nil
}
