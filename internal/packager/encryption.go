package packager

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

// EncryptionInfo holds the encryption keys and metadata for Detection.xml
type EncryptionInfo struct {
	EncryptionKey        []byte // 32-byte AES-256 key
	MacKey               []byte // 32-byte HMAC key
	InitializationVector []byte // 16-byte IV
	Mac                  []byte // 32-byte HMAC result
	FileDigest           []byte // SHA256 of original content
}

// GenerateKeys creates cryptographically secure random keys for encryption
// Returns: 32-byte encryption key, 32-byte MAC key, 16-byte IV
func GenerateKeys() (encKey, macKey, iv []byte, err error) {
	encKey = make([]byte, 32) // AES-256 requires 32-byte key
	macKey = make([]byte, 32) // HMAC-SHA256 uses 32-byte key
	iv = make([]byte, 16)     // AES block size is 16 bytes

	if _, err = rand.Read(encKey); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	if _, err = rand.Read(macKey); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate MAC key: %w", err)
	}

	if _, err = rand.Read(iv); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	return encKey, macKey, iv, nil
}

// PKCS7Pad adds PKCS7 padding to data to match the specified block size
// Block size for AES is always 16 bytes
func PKCS7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padBytes := make([]byte, padding)
	for i := range padBytes {
		padBytes[i] = byte(padding)
	}
	return append(data, padBytes...)
}

// PKCS7Unpad removes PKCS7 padding from data
func PKCS7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	padding := int(data[len(data)-1])
	if padding > len(data) || padding > aes.BlockSize {
		return nil, fmt.Errorf("invalid padding")
	}

	// Verify all padding bytes are correct
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}

	return data[:len(data)-padding], nil
}

// EncryptContent encrypts plaintext using AES-256-CBC and returns the blob
// Output format: [HMAC-SHA256 (32 bytes)][IV (16 bytes)][AES-256-CBC Ciphertext]
// This matches Microsoft's .intunewin encryption format
func EncryptContent(plaintext, encKey, macKey, iv []byte) ([]byte, error) {
	if len(encKey) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(encKey))
	}
	if len(macKey) != 32 {
		return nil, fmt.Errorf("MAC key must be 32 bytes, got %d", len(macKey))
	}
	if len(iv) != 16 {
		return nil, fmt.Errorf("IV must be 16 bytes, got %d", len(iv))
	}

	// Step 1: PKCS7 pad the plaintext
	padded := PKCS7Pad(plaintext, aes.BlockSize)

	// Step 2: Create AES cipher
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Step 3: Encrypt using CBC mode
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Step 4: Prepend IV to ciphertext
	ivAndCiphertext := append(iv, ciphertext...)

	// Step 5: Calculate HMAC-SHA256 over IV+ciphertext
	mac := hmac.New(sha256.New, macKey)
	mac.Write(ivAndCiphertext)
	hmacResult := mac.Sum(nil)

	// Step 6: Final format: [HMAC(32)][IV(16)][Ciphertext]
	result := append(hmacResult, ivAndCiphertext...)

	return result, nil
}

// DecryptContent decrypts data in the .intunewin format
// Input format: [HMAC-SHA256 (32 bytes)][IV (16 bytes)][AES-256-CBC Ciphertext]
func DecryptContent(encrypted, encKey, macKey []byte) ([]byte, error) {
	if len(encrypted) < 48 { // 32 (HMAC) + 16 (IV) minimum
		return nil, fmt.Errorf("encrypted data too short")
	}

	// Extract components
	hmacReceived := encrypted[:32]
	iv := encrypted[32:48]
	ciphertext := encrypted[48:]

	// Verify HMAC
	mac := hmac.New(sha256.New, macKey)
	mac.Write(encrypted[32:]) // IV + ciphertext
	hmacCalculated := mac.Sum(nil)

	if !hmac.Equal(hmacReceived, hmacCalculated) {
		return nil, fmt.Errorf("HMAC verification failed")
	}

	// Decrypt
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove padding
	return PKCS7Unpad(plaintext)
}

// CalculateFileDigest computes SHA256 hash of the data
func CalculateFileDigest(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// CreateEncryptionInfo generates all encryption components and returns EncryptionInfo
// This is the main entry point for encrypting content
func CreateEncryptionInfo(plaintext []byte) (*EncryptionInfo, []byte, error) {
	// Calculate file digest before encryption
	fileDigest := CalculateFileDigest(plaintext)

	// Generate keys
	encKey, macKey, iv, err := GenerateKeys()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	// Encrypt content
	encrypted, err := EncryptContent(plaintext, encKey, macKey, iv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt content: %w", err)
	}

	// Extract MAC from encrypted result (first 32 bytes)
	mac := encrypted[:32]

	info := &EncryptionInfo{
		EncryptionKey:        encKey,
		MacKey:               macKey,
		InitializationVector: iv,
		Mac:                  mac,
		FileDigest:           fileDigest,
	}

	return info, encrypted, nil
}
