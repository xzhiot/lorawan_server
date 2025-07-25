package lorawan

import (
	"crypto/aes"
	"encoding/binary"
	"fmt"
)

// SetUplinkDataMIC calculates and sets uplink MIC according to LoRaWAN spec
func (p *PHYPayload) SetUplinkDataMIC(version Major, confFCnt uint32, txDR, txCH byte, fNwkSIntKey, sNwkSIntKey AES128Key) error {
	// Parse MAC payload to get DevAddr and FCnt
	macPayload := &MACPayload{}
	if err := macPayload.Unmarshal(p.MACPayload, p.MHDR.MType, true); err != nil {
		return fmt.Errorf("unmarshal MAC payload: %w", err)
	}

	// Build B0 block for MIC calculation
	b0 := make([]byte, 16)
	b0[0] = 0x49 // Authentication flags
	b0[1] = 0x00
	b0[2] = 0x00
	b0[3] = 0x00
	b0[4] = 0x00
	b0[5] = 0x00 // Dir = 0 for uplink

	// DevAddr (4 bytes)
	copy(b0[6:10], macPayload.FHDR.DevAddr[:])

	// FCntUp (4 bytes)
	fullFCnt := GetFullFCnt(confFCnt, macPayload.FHDR.FCnt)
	binary.LittleEndian.PutUint32(b0[10:14], fullFCnt)

	// Zeros
	b0[14] = 0x00

	// Len
	b0[15] = byte(1 + len(p.MACPayload)) // MHDR + MACPayload

	// Build complete message for MIC
	micPayload := make([]byte, 0, len(b0)+1+len(p.MACPayload))
	micPayload = append(micPayload, b0...)
	micPayload = append(micPayload, byte(p.MHDR.MType<<5)|byte(p.MHDR.Major))
	micPayload = append(micPayload, p.MACPayload...)

	// Calculate MIC
	mic, err := aesCMACPRF(fNwkSIntKey[:], micPayload)
	if err != nil {
		return fmt.Errorf("calculate MIC: %w", err)
	}

	// Use first 4 bytes as MIC
	copy(p.MIC[:], mic[0:4])

	return nil
}

// SetDownlinkDataMIC sets downlink MIC according to LoRaWAN spec
func (p *PHYPayload) SetDownlinkDataMIC(version Major, confFCnt uint32, sNwkSIntKey AES128Key) error {
	// Parse MAC payload
	macPayload := &MACPayload{}
	if err := macPayload.Unmarshal(p.MACPayload, p.MHDR.MType, false); err != nil {
		return fmt.Errorf("unmarshal MAC payload: %w", err)
	}

	// Build B0 block
	b0 := make([]byte, 16)
	b0[0] = 0x49
	b0[1] = 0x00
	b0[2] = 0x00
	b0[3] = 0x00
	b0[4] = 0x00
	b0[5] = 0x01 // Dir = 1 for downlink

	// DevAddr
	copy(b0[6:10], macPayload.FHDR.DevAddr[:])

	// FCntDown - use confFCnt directly for downlink
	binary.LittleEndian.PutUint32(b0[10:14], confFCnt)

	b0[14] = 0x00
	b0[15] = byte(1 + len(p.MACPayload))

	// Build complete message
	micPayload := make([]byte, 0, len(b0)+1+len(p.MACPayload))
	micPayload = append(micPayload, b0...)
	micPayload = append(micPayload, byte(p.MHDR.MType<<5)|byte(p.MHDR.Major))
	micPayload = append(micPayload, p.MACPayload...)

	// Calculate MIC
	mic, err := aesCMACPRF(sNwkSIntKey[:], micPayload)
	if err != nil {
		return fmt.Errorf("calculate MIC: %w", err)
	}

	copy(p.MIC[:], mic[0:4])

	return nil
}

// ValidateUplinkDataMIC validates uplink MIC
func (p *PHYPayload) ValidateUplinkDataMIC(version Major, confFCnt uint32, txDR, txCH byte, fNwkSIntKey, sNwkSIntKey AES128Key) (bool, error) {
	// Save original MIC
	origMIC := p.MIC

	// Calculate expected MIC
	if err := p.SetUplinkDataMIC(version, confFCnt, txDR, txCH, fNwkSIntKey, sNwkSIntKey); err != nil {
		return false, err
	}

	// Compare
	valid := p.MIC == origMIC

	// Restore original MIC
	p.MIC = origMIC

	return valid, nil
}

// ValidateUplinkJoinMIC validates JOIN REQUEST MIC
func (p *PHYPayload) ValidateUplinkJoinMIC(appKey AES128Key) (bool, error) {
	// JOIN REQUEST MIC calculation according to LoRaWAN spec
	// MIC = aes128_cmac(AppKey, MHDR | JoinEUI | DevEUI | DevNonce)

	// Build data for MIC calculation
	var data []byte

	// Add MHDR
	mhdrByte := byte(p.MHDR.MType<<5) | byte(p.MHDR.Major)
	data = append(data, mhdrByte)

	// Add MAC payload (JoinRequest content)
	data = append(data, p.MACPayload...)

	// Calculate expected MIC
	expectedMIC, err := CalculateMIC(appKey[:], data)
	if err != nil {
		return false, fmt.Errorf("calculate JOIN REQUEST MIC: %w", err)
	}

	// Compare with received MIC
	return expectedMIC == p.MIC, nil
}

// UnmarshalBinary unmarshals PHYPayload from binary
func (p *PHYPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("PHYPayload too short: %d bytes", len(data))
	}

	// MHDR
	p.MHDR.MType = MType((data[0] >> 5) & 0x07)
	p.MHDR.Major = Major(data[0] & 0x03)

	// MACPayload
	p.MACPayload = data[1 : len(data)-4]

	// MIC
	copy(p.MIC[:], data[len(data)-4:])

	return nil
}

// GetFullFCnt gets full frame counter from 16-bit value
func GetFullFCnt(fCntUp uint32, fCnt uint16) uint32 {
	// Get the upper 16 bits from fCntUp
	upperBits := fCntUp & 0xFFFF0000

	// Check for rollover
	if uint16(fCntUp) > fCnt && (uint16(fCntUp)-fCnt) > 0x8000 {
		// Rollover occurred
		upperBits += 0x10000
	}

	return upperBits | uint32(fCnt)
}

// EncryptFRMPayload encrypts/decrypts FRM payload
func EncryptFRMPayload(key []byte, devAddr DevAddr, fCnt uint32, uplink bool, payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return payload, nil
	}

	// Calculate number of blocks
	k := (len(payload) + 15) / 16

	// Build Ai blocks
	ai := make([]byte, 16)
	ai[0] = 0x01
	ai[1] = 0x00
	ai[2] = 0x00
	ai[3] = 0x00
	ai[4] = 0x00

	if uplink {
		ai[5] = 0x00 // Dir = 0 for uplink
	} else {
		ai[5] = 0x01 // Dir = 1 for downlink
	}

	// DevAddr
	copy(ai[6:10], devAddr[:])

	// FCnt
	binary.LittleEndian.PutUint32(ai[10:14], fCnt)

	ai[14] = 0x00

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Generate keystream
	s := make([]byte, 16*k)
	for i := 0; i < k; i++ {
		ai[15] = byte(i + 1)
		block.Encrypt(s[i*16:(i+1)*16], ai)
	}

	// XOR with payload
	encrypted := make([]byte, len(payload))
	for i := range payload {
		encrypted[i] = payload[i] ^ s[i]
	}

	return encrypted, nil
}

// Marshal marshals MACPayload
func (m *MACPayload) Marshal(mtype MType, isUplink bool) ([]byte, error) {
	var data []byte

	// DevAddr
	data = append(data, m.FHDR.DevAddr[:]...)

	// FCtrl
	fctrl := byte(0)
	if m.FHDR.FCtrl.ADR {
		fctrl |= 0x80
	}
	if isUplink {
		if m.FHDR.FCtrl.ADRACKReq {
			fctrl |= 0x40
		}
		if m.FHDR.FCtrl.ACK {
			fctrl |= 0x20
		}
		if m.FHDR.FCtrl.ClassB {
			fctrl |= 0x10
		}
	} else {
		if m.FHDR.FCtrl.ACK {
			fctrl |= 0x20
		}
		if m.FHDR.FCtrl.FPending {
			fctrl |= 0x10
		}
	}
	fctrl |= byte(len(m.FHDR.FOpts)) & 0x0F
	data = append(data, fctrl)

	// FCnt (16-bit)
	data = append(data, byte(m.FHDR.FCnt), byte(m.FHDR.FCnt>>8))

	// FOpts
	data = append(data, m.FHDR.FOpts...)

	// FPort (optional)
	if m.FPort != nil {
		data = append(data, *m.FPort)
		// FRMPayload only present if FPort is present
		data = append(data, m.FRMPayload...)
	}

	return data, nil
}

// Unmarshal unmarshals MACPayload
func (m *MACPayload) Unmarshal(data []byte, mtype MType, isUplink bool) error {
	if len(data) < 7 {
		return fmt.Errorf("MACPayload too short: %d bytes", len(data))
	}

	pos := 0

	// DevAddr (4 bytes)
	copy(m.FHDR.DevAddr[:], data[pos:pos+4])
	pos += 4

	// FCtrl (1 byte)
	fctrl := data[pos]
	m.FHDR.FCtrl.ADR = (fctrl & 0x80) != 0
	if isUplink {
		m.FHDR.FCtrl.ADRACKReq = (fctrl & 0x40) != 0
		m.FHDR.FCtrl.ACK = (fctrl & 0x20) != 0
		m.FHDR.FCtrl.ClassB = (fctrl & 0x10) != 0
	} else {
		m.FHDR.FCtrl.ACK = (fctrl & 0x20) != 0
		m.FHDR.FCtrl.FPending = (fctrl & 0x10) != 0
	}
	foptsLen := int(fctrl & 0x0F)
	pos++

	// FCnt (2 bytes)
	m.FHDR.FCnt = uint16(data[pos]) | uint16(data[pos+1])<<8
	pos += 2

	// FOpts (variable length)
	if foptsLen > 0 {
		if pos+foptsLen > len(data) {
			return fmt.Errorf("invalid FOpts length")
		}
		m.FHDR.FOpts = data[pos : pos+foptsLen]
		pos += foptsLen
	}

	// FPort and FRMPayload (optional)
	if pos < len(data) {
		fport := data[pos]
		m.FPort = &fport
		pos++

		if pos < len(data) {
			m.FRMPayload = data[pos:]
		}
	}

	return nil
}

// JoinRequest/Accept marshal/unmarshal methods
func (j *JoinRequestPayload) UnmarshalBinary(data []byte) error {
	if len(data) != 18 {
		return fmt.Errorf("invalid JoinRequest length: expected 18, got %d", len(data))
	}

	copy(j.JoinEUI[:], data[0:8])
	copy(j.DevEUI[:], data[8:16])
	copy(j.DevNonce[:], data[16:18])

	return nil
}

func (j *JoinAcceptPayload) MarshalBinary() ([]byte, error) {
	size := 12
	if len(j.CFList) > 0 {
		size += len(j.CFList)
	}

	data := make([]byte, size)
	copy(data[0:3], j.JoinNonce[:])
	copy(data[3:6], j.NetID[:])
	copy(data[6:10], j.DevAddr[:])
	data[10] = (j.DLSettings.RX1DROffset << 4) | (j.DLSettings.RX2DataRate & 0x0F)
	data[11] = j.RxDelay

	if len(j.CFList) > 0 {
		copy(data[12:], j.CFList)
	}

	return data, nil
}

func (j *JoinAcceptPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("invalid JoinAccept length: minimum 12, got %d", len(data))
	}

	copy(j.JoinNonce[:], data[0:3])
	copy(j.NetID[:], data[3:6])
	copy(j.DevAddr[:], data[6:10])
	j.DLSettings.RX1DROffset = (data[10] >> 4) & 0x07
	j.DLSettings.RX2DataRate = data[10] & 0x0F
	j.RxDelay = data[11]

	if len(data) > 12 {
		j.CFList = make([]byte, len(data)-12)
		copy(j.CFList, data[12:])
	}

	return nil
}

// CalculateMIC is a helper function to calculate MIC
func CalculateMIC(key []byte, data []byte) ([4]byte, error) {
	var mic [4]byte
	hash, err := aesCMACPRF(key, data)
	if err != nil {
		return mic, err
	}
	copy(mic[:], hash[0:4])
	return mic, nil
}

// SetJoinAcceptMIC sets the MIC for Join Accept message
func (p *PHYPayload) SetJoinAcceptMIC(key AES128Key) error {
	// JOIN ACCEPT MIC calculation according to LoRaWAN 1.0.3 spec
	// MIC = aes128_cmac(AppKey, MHDR | JoinAccept)

	// Build data for MIC calculation
	var data []byte

	// Add MHDR - 注意这里的修正
	mhdrByte := byte(p.MHDR.MType<<5) | byte(p.MHDR.Major)
	data = append(data, mhdrByte)

	// Add MAC payload (JoinAccept content)
	data = append(data, p.MACPayload...)

	// Calculate MIC using existing function
	mic, err := CalculateMIC(key[:], data)
	if err != nil {
		return fmt.Errorf("calculate JOIN ACCEPT MIC: %w", err)
	}

	// Set MIC
	p.MIC = mic

	return nil
}

// EncryptJoinAcceptPayload encrypts Join Accept payload using AES-ECB
func (p *PHYPayload) EncryptJoinAcceptPayload(key AES128Key) error {
	// JOIN ACCEPT encryption according to LoRaWAN 1.0.3 spec
	// 重要：使用 AES DECRYPT 操作来加密！
	// aes128_decrypt(AppKey, JoinAccept | MIC)

	// Prepare data to encrypt: MACPayload + MIC
	plaintext := make([]byte, len(p.MACPayload)+4)
	copy(plaintext, p.MACPayload)
	copy(plaintext[len(p.MACPayload):], p.MIC[:])

	// 使用 AES DECRYPT 操作来加密（LoRaWAN 特殊要求）
	ciphertext, err := aesECBDecrypt(key[:], plaintext)
	if err != nil {
		return fmt.Errorf("encrypt JOIN ACCEPT: %w", err)
	}

	// 重要：加密后的数据直接作为 MACPayload，不再分离 MIC
	p.MACPayload = ciphertext
	// MIC 已经包含在加密的 MACPayload 中，不需要单独存储

	return nil
}

// MarshalBinary marshals PHYPayload to binary - 修正版
func (p *PHYPayload) MarshalBinary() ([]byte, error) {
	var data []byte

	// MHDR
	mhdr := byte(p.MHDR.MType<<5) | byte(p.MHDR.Major)
	data = append(data, mhdr)

	// MACPayload
	data = append(data, p.MACPayload...)

	// 对于 JOIN ACCEPT，MIC 已经包含在加密的 MACPayload 中
	// 所以不需要再次附加 MIC
	if p.MHDR.MType != JoinAccept && len(p.MIC) > 0 {
		// 其他消息类型需要附加 MIC
		data = append(data, p.MIC[:]...)
	}

	return data, nil
}

// 添加 aesECBDecrypt 函数（用于 JOIN ACCEPT 加密）
func aesECBDecrypt(key []byte, data []byte) ([]byte, error) {
	// 确保数据长度是16的倍数
	if len(data)%aes.BlockSize != 0 {
		// JOIN ACCEPT 不需要填充，长度应该正好是16或32字节
		return nil, fmt.Errorf("invalid data length for AES ECB: %d", len(data))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// AES-ECB decryption (用于加密 JOIN ACCEPT)
	ciphertext := make([]byte, len(data))

	// Process in 16-byte blocks
	for i := 0; i < len(data); i += aes.BlockSize {
		// 使用 Decrypt 操作来加密（LoRaWAN 的特殊要求）
		block.Decrypt(ciphertext[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
	}

	return ciphertext, nil
}

// 为了调试，添加一个辅助函数
func (p *PHYPayload) DebugJoinAccept() string {
	if p.MHDR.MType != JoinAccept {
		return "Not a JOIN ACCEPT"
	}

	result := fmt.Sprintf("MHDR: %02X\n", byte(p.MHDR.MType<<5)|byte(p.MHDR.Major))
	result += fmt.Sprintf("MACPayload: %X\n", p.MACPayload)
	result += fmt.Sprintf("Total Length: %d\n", 1+len(p.MACPayload))

	return result
}
