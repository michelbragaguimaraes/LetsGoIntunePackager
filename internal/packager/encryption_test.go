package packager

import (
	"bytes"
	"crypto/aes"
	"testing"
)

func TestGenerateKeys(t *testing.T) {
	encKey, macKey, iv, err := GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys() error = %v", err)
	}

	// Verify key sizes
	if len(encKey) != 32 {
		t.Errorf("encKey length = %d, want 32", len(encKey))
	}
	if len(macKey) != 32 {
		t.Errorf("macKey length = %d, want 32", len(macKey))
	}
	if len(iv) != 16 {
		t.Errorf("iv length = %d, want 16", len(iv))
	}

	// Verify keys are not all zeros (randomness check)
	allZeros := make([]byte, 32)
	if bytes.Equal(encKey, allZeros) {
		t.Error("encKey is all zeros")
	}
	if bytes.Equal(macKey, allZeros) {
		t.Error("macKey is all zeros")
	}
	allZerosIV := make([]byte, 16)
	if bytes.Equal(iv, allZerosIV) {
		t.Error("iv is all zeros")
	}

	// Verify keys are different from each other
	if bytes.Equal(encKey, macKey) {
		t.Error("encKey and macKey are identical")
	}
}

func TestPKCS7Pad(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		blockSize int
		wantLen   int
	}{
		{
			name:      "empty input",
			input:     []byte{},
			blockSize: aes.BlockSize,
			wantLen:   aes.BlockSize,
		},
		{
			name:      "1 byte input",
			input:     []byte{0x01},
			blockSize: aes.BlockSize,
			wantLen:   aes.BlockSize,
		},
		{
			name:      "15 byte input",
			input:     bytes.Repeat([]byte{0x01}, 15),
			blockSize: aes.BlockSize,
			wantLen:   aes.BlockSize,
		},
		{
			name:      "16 byte input (full block)",
			input:     bytes.Repeat([]byte{0x01}, 16),
			blockSize: aes.BlockSize,
			wantLen:   aes.BlockSize * 2, // Need full padding block
		},
		{
			name:      "17 byte input",
			input:     bytes.Repeat([]byte{0x01}, 17),
			blockSize: aes.BlockSize,
			wantLen:   aes.BlockSize * 2,
		},
		{
			name:      "32 byte input",
			input:     bytes.Repeat([]byte{0x01}, 32),
			blockSize: aes.BlockSize,
			wantLen:   aes.BlockSize * 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PKCS7Pad(tt.input, tt.blockSize)
			if len(result) != tt.wantLen {
				t.Errorf("PKCS7Pad() length = %d, want %d", len(result), tt.wantLen)
			}
			// Verify result is multiple of block size
			if len(result)%tt.blockSize != 0 {
				t.Errorf("PKCS7Pad() result not multiple of block size")
			}
			// Verify padding value is correct
			paddingLen := int(result[len(result)-1])
			if paddingLen > tt.blockSize || paddingLen == 0 {
				t.Errorf("Invalid padding value: %d", paddingLen)
			}
			// Verify all padding bytes are correct
			for i := len(result) - paddingLen; i < len(result); i++ {
				if result[i] != byte(paddingLen) {
					t.Errorf("Padding byte at %d = %d, want %d", i, result[i], paddingLen)
				}
			}
		})
	}
}

func TestEncryptContent(t *testing.T) {
	encKey, macKey, iv, err := GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys() error = %v", err)
	}

	plaintext := []byte("Hello, Microsoft Intune!")

	encrypted, err := EncryptContent(plaintext, encKey, macKey, iv)
	if err != nil {
		t.Fatalf("EncryptContent() error = %v", err)
	}

	// Verify output format: [HMAC(32)][IV(16)][Ciphertext]
	if len(encrypted) < 32+16 {
		t.Fatalf("encrypted length = %d, want >= 48", len(encrypted))
	}

	// Verify ciphertext is multiple of block size
	ciphertextLen := len(encrypted) - 32 - 16
	if ciphertextLen%aes.BlockSize != 0 {
		t.Errorf("ciphertext length %d is not multiple of block size", ciphertextLen)
	}

	// Verify IV is in correct position
	extractedIV := encrypted[32:48]
	if !bytes.Equal(extractedIV, iv) {
		t.Error("IV not in correct position")
	}
}

func TestCalculateFileDigest(t *testing.T) {
	data := []byte("test data for hashing")
	digest := CalculateFileDigest(data)

	// SHA256 produces 32-byte hash
	if len(digest) != 32 {
		t.Errorf("digest length = %d, want 32", len(digest))
	}

	// Same input should produce same hash
	digest2 := CalculateFileDigest(data)
	if !bytes.Equal(digest, digest2) {
		t.Error("same input produced different hashes")
	}

	// Different input should produce different hash
	digest3 := CalculateFileDigest([]byte("different data"))
	if bytes.Equal(digest, digest3) {
		t.Error("different input produced same hash")
	}
}

func TestCreateEncryptionInfo(t *testing.T) {
	data := []byte("test content for encryption")

	info, encryptedData, err := CreateEncryptionInfo(data)
	if err != nil {
		t.Fatalf("CreateEncryptionInfo() error = %v", err)
	}

	// Verify all fields are populated
	if len(info.EncryptionKey) == 0 {
		t.Error("EncryptionKey is empty")
	}
	if len(info.MacKey) == 0 {
		t.Error("MacKey is empty")
	}
	if len(info.InitializationVector) == 0 {
		t.Error("InitializationVector is empty")
	}
	if len(encryptedData) == 0 {
		t.Error("encryptedData is empty")
	}
	if len(info.FileDigest) == 0 {
		t.Error("FileDigest is empty")
	}
	if len(info.Mac) == 0 {
		t.Error("Mac is empty")
	}
}

func TestEncryptionRoundTrip(t *testing.T) {
	// Generate keys
	encKey, macKey, iv, err := GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys() error = %v", err)
	}

	// Test various input sizes
	testCases := []struct {
		name string
		size int
	}{
		{"small", 10},
		{"medium", 1024},
		{"large", 65536},
		{"block_aligned", 16 * 100},
		{"block_unaligned", 16*100 + 7},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plaintext := bytes.Repeat([]byte{0xAB}, tc.size)

			encrypted, err := EncryptContent(plaintext, encKey, macKey, iv)
			if err != nil {
				t.Fatalf("EncryptContent() error = %v", err)
			}

			// Verify encrypted data is larger than plaintext (due to padding)
			if len(encrypted) <= len(plaintext) {
				t.Errorf("encrypted (%d) should be larger than plaintext (%d)", len(encrypted), len(plaintext))
			}

			// Verify format
			if len(encrypted) < 48 {
				t.Errorf("encrypted data too short: %d", len(encrypted))
			}
		})
	}
}
