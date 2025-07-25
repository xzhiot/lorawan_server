package lorawan

import (
    "crypto/aes"
    "encoding/binary"
)

// DeriveSessionKeys derives session keys according to LoRaWAN 1.0.x spec
func DeriveSessionKeys10(appKey []byte, appNonce [3]byte, netID [3]byte, devNonce [2]byte) (nwkSKey, appSKey [16]byte, err error) {
    // NwkSKey = aes128_encrypt(AppKey, 0x01 | AppNonce | NetID | DevNonce | pad16)
    nwkSKeyMsg := make([]byte, 16)
    nwkSKeyMsg[0] = 0x01
    copy(nwkSKeyMsg[1:4], appNonce[:])
    copy(nwkSKeyMsg[4:7], netID[:])
    copy(nwkSKeyMsg[7:9], devNonce[:])
    
    block, err := aes.NewCipher(appKey)
    if err != nil {
        return nwkSKey, appSKey, err
    }
    
    block.Encrypt(nwkSKey[:], nwkSKeyMsg)
    
    // AppSKey = aes128_encrypt(AppKey, 0x02 | AppNonce | NetID | DevNonce | pad16)
    appSKeyMsg := make([]byte, 16)
    appSKeyMsg[0] = 0x02
    copy(appSKeyMsg[1:4], appNonce[:])
    copy(appSKeyMsg[4:7], netID[:])
    copy(appSKeyMsg[7:9], devNonce[:])
    
    block.Encrypt(appSKey[:], appSKeyMsg)
    
    return nwkSKey, appSKey, nil
}

// DeriveSessionKeys11 derives session keys according to LoRaWAN 1.1 spec
func DeriveSessionKeys11(nwkKey, appKey []byte, joinNonce [3]byte, joinEUI [8]byte, devNonce [2]byte) (
    appSKey, fNwkSIntKey, sNwkSIntKey, nwkSEncKey [16]byte, err error) {
    
    // AppSKey = aes128_encrypt(AppKey, 0x02 | JoinNonce | JoinEUI | DevNonce | pad16)
    appSKeyMsg := make([]byte, 16)
    appSKeyMsg[0] = 0x02
    copy(appSKeyMsg[1:4], joinNonce[:])
    copy(appSKeyMsg[4:12], joinEUI[:])
    binary.LittleEndian.PutUint16(appSKeyMsg[12:14], binary.LittleEndian.Uint16(devNonce[:]))
    
    block, err := aes.NewCipher(appKey)
    if err != nil {
        return
    }
    block.Encrypt(appSKey[:], appSKeyMsg)
    
    // FNwkSIntKey = aes128_encrypt(NwkKey, 0x01 | JoinNonce | JoinEUI | DevNonce | pad16)
    fNwkSIntKeyMsg := make([]byte, 16)
    fNwkSIntKeyMsg[0] = 0x01
    copy(fNwkSIntKeyMsg[1:4], joinNonce[:])
    copy(fNwkSIntKeyMsg[4:12], joinEUI[:])
    binary.LittleEndian.PutUint16(fNwkSIntKeyMsg[12:14], binary.LittleEndian.Uint16(devNonce[:]))
    
    block, err = aes.NewCipher(nwkKey)
    if err != nil {
        return
    }
    block.Encrypt(fNwkSIntKey[:], fNwkSIntKeyMsg)
    
    // SNwkSIntKey = aes128_encrypt(NwkKey, 0x03 | JoinNonce | JoinEUI | DevNonce | pad16)
    sNwkSIntKeyMsg := make([]byte, 16)
    sNwkSIntKeyMsg[0] = 0x03
    copy(sNwkSIntKeyMsg[1:4], joinNonce[:])
    copy(sNwkSIntKeyMsg[4:12], joinEUI[:])
    binary.LittleEndian.PutUint16(sNwkSIntKeyMsg[12:14], binary.LittleEndian.Uint16(devNonce[:]))
    
    block.Encrypt(sNwkSIntKey[:], sNwkSIntKeyMsg)
    
    // NwkSEncKey = aes128_encrypt(NwkKey, 0x04 | JoinNonce | JoinEUI | DevNonce | pad16)
    nwkSEncKeyMsg := make([]byte, 16)
    nwkSEncKeyMsg[0] = 0x04
    copy(nwkSEncKeyMsg[1:4], joinNonce[:])
    copy(nwkSEncKeyMsg[4:12], joinEUI[:])
    binary.LittleEndian.PutUint16(nwkSEncKeyMsg[12:14], binary.LittleEndian.Uint16(devNonce[:]))
    
    block.Encrypt(nwkSEncKey[:], nwkSEncKeyMsg)
    
    return
}

// EncryptJoinAccept encrypts join accept payload
func EncryptJoinAccept(key []byte, payload []byte) ([]byte, error) {
    if len(payload)%16 != 0 {
        // Pad to 16 bytes
        padding := 16 - (len(payload) % 16)
        payload = append(payload, make([]byte, padding)...)
    }
    
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    encrypted := make([]byte, len(payload))
    for i := 0; i < len(payload); i += 16 {
        block.Decrypt(encrypted[i:i+16], payload[i:i+16]) // Note: using Decrypt for encryption in ECB mode
    }
    
    return encrypted, nil
}

// DecryptJoinAccept decrypts join accept payload
func DecryptJoinAccept(key []byte, encrypted []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    decrypted := make([]byte, len(encrypted))
    for i := 0; i < len(encrypted); i += 16 {
        block.Encrypt(decrypted[i:i+16], encrypted[i:i+16]) // Note: using Encrypt for decryption in ECB mode
    }
    
    return decrypted, nil
}
