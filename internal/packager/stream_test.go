package packager

import (
	"bytes"
	"crypto/aes"
	"math/rand"
	"testing"
)

// fixedKeys returns deterministic key material so streamed and buffered
// encryption of the same payload can be compared byte for byte.
func fixedKeys() (encKey, macKey, iv []byte) {
	encKey = bytes.Repeat([]byte{0xA5}, 32)
	macKey = bytes.Repeat([]byte{0x5A}, 32)
	iv = bytes.Repeat([]byte{0x3C}, aes.BlockSize)
	return
}

// streamEncrypt runs plaintext through the streaming encryptor, writing it in
// chunks of the given size, and returns the blob in .intunewin layout.
func streamEncrypt(t *testing.T, plaintext []byte, chunk int) []byte {
	t.Helper()
	encKey, macKey, iv := fixedKeys()

	var body bytes.Buffer
	enc, err := newContentEncryptor(&body, encKey, macKey, iv)
	if err != nil {
		t.Fatalf("newContentEncryptor() error = %v", err)
	}

	for off := 0; off < len(plaintext); {
		end := off + chunk
		if end > len(plaintext) {
			end = len(plaintext)
		}
		if _, err := enc.Write(plaintext[off:end]); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		off = end
	}

	mac, err := enc.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return append(mac, body.Bytes()...)
}

// interestingSizes covers the block and chunk boundaries the writer has to get
// right: empty input, either side of a 16-byte AES block, and either side of
// the 1 MiB ciphertext chunk.
func interestingSizes() []int {
	return []int{
		0, 1, 15, 16, 17, 31, 32, 33, 255, 256, 257,
		1023, 1024, 1025, 4095, 4096, 4097,
		cbcChunkSize - 1, cbcChunkSize, cbcChunkSize + 1,
		cbcChunkSize + aes.BlockSize, 2*cbcChunkSize + 7,
	}
}

func TestStreamingMatchesBufferedEncryption(t *testing.T) {
	encKey, macKey, iv := fixedKeys()
	rng := rand.New(rand.NewSource(1))

	for _, size := range interestingSizes() {
		plaintext := make([]byte, size)
		rng.Read(plaintext)

		want, err := EncryptContent(plaintext, encKey, macKey, iv)
		if err != nil {
			t.Fatalf("EncryptContent(%d bytes) error = %v", size, err)
		}
		got := streamEncrypt(t, plaintext, len(plaintext)+1)

		if !bytes.Equal(got, want) {
			t.Errorf("size %d: streamed blob differs from buffered blob (%d vs %d bytes)",
				size, len(got), len(want))
		}
	}
}

func TestStreamingIndependentOfWriteChunking(t *testing.T) {
	// How the caller splits its writes must not change a single output byte.
	// archive/zip writes in whatever sizes it likes, so this is the property
	// the packaging path depends on.
	rng := rand.New(rand.NewSource(2))
	plaintext := make([]byte, 3*cbcChunkSize+12345)
	rng.Read(plaintext)

	encKey, macKey, iv := fixedKeys()
	want, err := EncryptContent(plaintext, encKey, macKey, iv)
	if err != nil {
		t.Fatalf("EncryptContent() error = %v", err)
	}

	for _, chunk := range []int{1, 3, 15, 16, 17, 64, 4096, cbcChunkSize - 1, cbcChunkSize, cbcChunkSize + 1, len(plaintext)} {
		if got := streamEncrypt(t, plaintext, chunk); !bytes.Equal(got, want) {
			t.Errorf("chunk size %d produced a different blob", chunk)
		}
	}
}

func TestStreamingRoundTrips(t *testing.T) {
	encKey, macKey, _ := fixedKeys()
	rng := rand.New(rand.NewSource(3))

	for _, size := range interestingSizes() {
		plaintext := make([]byte, size)
		rng.Read(plaintext)

		blob := streamEncrypt(t, plaintext, 997) // deliberately not block-aligned
		got, err := DecryptContent(blob, encKey, macKey)
		if err != nil {
			t.Errorf("size %d: DecryptContent() error = %v", size, err)
			continue
		}
		if !bytes.Equal(got, plaintext) {
			t.Errorf("size %d: round trip returned %d bytes, want %d", size, len(got), size)
		}
	}
}

func TestContentEncryptorRejectsBadKeyMaterial(t *testing.T) {
	encKey, macKey, iv := fixedKeys()

	for _, tt := range []struct {
		name         string
		enc, mac, iv []byte
	}{
		{"short encryption key", encKey[:16], macKey, iv},
		{"short MAC key", encKey, macKey[:16], iv},
		{"short IV", encKey, macKey, iv[:8]},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := newContentEncryptor(&bytes.Buffer{}, tt.enc, tt.mac, tt.iv); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestCBCEncryptWriterRejectsWriteAfterClose(t *testing.T) {
	encKey, macKey, iv := fixedKeys()
	enc, err := newContentEncryptor(&bytes.Buffer{}, encKey, macKey, iv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Write([]byte("late")); err == nil {
		t.Error("expected an error writing after close, got nil")
	}
}

func TestCountingWriter(t *testing.T) {
	c := &countingWriter{}
	for _, s := range []string{"", "a", "bcdef"} {
		if n, err := c.Write([]byte(s)); err != nil || n != len(s) {
			t.Fatalf("Write(%q) = %d, %v", s, n, err)
		}
	}
	if c.n != 6 {
		t.Errorf("counted %d bytes, want 6", c.n)
	}
}
