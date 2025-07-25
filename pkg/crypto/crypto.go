package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// VerifyPassword verifies a password against a hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateRandomBytes generates random bytes
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

// GenerateRandomString generates a random string
func GenerateRandomString(n int) (string, error) {
	bytes, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// Encrypt encrypts data using AES-GCM
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-GCM
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// DecryptFRMPayload decrypts LoRaWAN FRM payload
func DecryptFRMPayload(key []byte, uplink bool, devAddr [4]byte, fCnt uint32, payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return payload, nil
	}

	k := len(payload) / 16
	if len(payload)%16 != 0 {
		k++
	}

	a := make([]byte, 16)
	a[0] = 0x01
	if !uplink {
		a[5] = 0x01
	}
	copy(a[6:10], devAddr[:])
	a[10] = byte(fCnt)
	a[11] = byte(fCnt >> 8)
	a[12] = byte(fCnt >> 16)
	a[13] = byte(fCnt >> 24)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	s := make([]byte, 16*k)
	for i := 0; i < k; i++ {
		a[15] = byte(i + 1)
		block.Encrypt(s[i*16:(i+1)*16], a)
	}

	decrypted := make([]byte, len(payload))
	for i := range payload {
		decrypted[i] = payload[i] ^ s[i]
	}

	return decrypted, nil
}
