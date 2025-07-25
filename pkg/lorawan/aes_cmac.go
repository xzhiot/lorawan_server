package lorawan

import (
    "crypto/aes"
    "crypto/cipher"
//    "encoding/binary"
)

// aesCMACPRF implements AES-CMAC-PRF-128 according to RFC 4493
func aesCMACPRF(key, data []byte) ([]byte, error) {
    // Create cipher
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    // Generate subkeys
    k1, k2 := generateSubkeys(block)
    
    // Process data
    n := len(data)
    flag := false
    
    var mLast []byte
    if n == 0 {
        // Special case for empty data
        mLast = make([]byte, 16)
        mLast[0] = 0x80
        flag = false
        for i := 0; i < 16; i++ {
            mLast[i] ^= k2[i]
        }
    } else {
        // Calculate number of blocks
        numBlocks := (n + 15) / 16
        
        if n%16 == 0 {
            // Last block is complete
            flag = true
            mLast = make([]byte, 16)
            copy(mLast, data[(numBlocks-1)*16:])
            for i := 0; i < 16; i++ {
                mLast[i] ^= k1[i]
            }
        } else {
            // Last block needs padding
            flag = false
            mLast = make([]byte, 16)
            remainder := n % 16
            copy(mLast, data[(numBlocks-1)*16:])
            mLast[remainder] = 0x80
            for i := 0; i < 16; i++ {
                mLast[i] ^= k2[i]
            }
        }
    }
    
    // Process all blocks
    x := make([]byte, 16)
    y := make([]byte, 16)
    
    // Process all but last block
    numBlocks := len(data) / 16
    if !flag && len(data)%16 == 0 && len(data) > 0 {
        numBlocks--
    }
    
    for i := 0; i < numBlocks; i++ {
        // XOR with previous result
        for j := 0; j < 16; j++ {
            y[j] = x[j] ^ data[i*16+j]
        }
        block.Encrypt(x, y)
    }
    
    // Process last block
    for j := 0; j < 16; j++ {
        y[j] = x[j] ^ mLast[j]
    }
    block.Encrypt(x, y)
    
    return x, nil
}

// generateSubkeys generates K1 and K2 for AES-CMAC
func generateSubkeys(block cipher.Block) (k1, k2 []byte) {
    // Constants
    const rb = 0x87
    
    k0 := make([]byte, 16)
    block.Encrypt(k0, make([]byte, 16))
    
    k1 = leftShift(k0)
    if k0[0]&0x80 != 0 {
        k1[15] ^= rb
    }
    
    k2 = leftShift(k1)
    if k1[0]&0x80 != 0 {
        k2[15] ^= rb
    }
    
    return k1, k2
}

// leftShift performs a left shift on a byte slice
func leftShift(b []byte) []byte {
    result := make([]byte, len(b))
    overflow := byte(0)
    
    for i := len(b) - 1; i >= 0; i-- {
        result[i] = b[i]<<1 | overflow
        overflow = (b[i] & 0x80) >> 7
    }
    
    return result
}
