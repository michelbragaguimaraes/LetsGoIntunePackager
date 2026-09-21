package packager

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"
)

// cbcChunkSize bounds how much ciphertext is produced per write to the
// underlying writer. It only affects syscall granularity, not the output.
const cbcChunkSize = 1 << 20

// cbcEncryptWriter encrypts everything written to it with AES-CBC and PKCS7
// padding, emitting ciphertext as it goes.
//
// It exists so a payload can be encrypted while it is still being produced,
// instead of being held in memory in full.
type cbcEncryptWriter struct {
	mode    cipher.BlockMode
	dst     io.Writer
	pending []byte // plaintext tail not yet forming a whole block
	scratch []byte
	closed  bool
}

func newCBCEncryptWriter(dst io.Writer, block cipher.Block, iv []byte) *cbcEncryptWriter {
	return &cbcEncryptWriter{
		mode:    cipher.NewCBCEncrypter(block, iv),
		dst:     dst,
		pending: make([]byte, 0, aes.BlockSize),
		scratch: make([]byte, cbcChunkSize),
	}
}

func (w *cbcEncryptWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errors.New("write after close")
	}
	total := len(p)

	// Complete the partial block carried over from an earlier write.
	if len(w.pending) > 0 {
		need := aes.BlockSize - len(w.pending)
		if need > len(p) {
			w.pending = append(w.pending, p...)
			return total, nil
		}
		w.pending = append(w.pending, p[:need]...)
		p = p[need:]
		if err := w.encrypt(w.pending); err != nil {
			return 0, err
		}
		w.pending = w.pending[:0]
	}

	// Encrypt every whole block p holds, in bounded chunks.
	whole := len(p) - len(p)%aes.BlockSize
	for off := 0; off < whole; {
		end := off + len(w.scratch)
		if end > whole {
			end = whole
		}
		if err := w.encrypt(p[off:end]); err != nil {
			return 0, err
		}
		off = end
	}

	w.pending = append(w.pending, p[whole:]...)
	return total, nil
}

// encrypt transforms one or more whole blocks and writes the ciphertext.
func (w *cbcEncryptWriter) encrypt(blocks []byte) error {
	buf := w.scratch[:len(blocks)]
	w.mode.CryptBlocks(buf, blocks)
	_, err := w.dst.Write(buf)
	return err
}

// Close emits the PKCS7-padded final block. The underlying writer is left open.
func (w *cbcEncryptWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	// PKCS7 always adds a block's worth at most, and pending is short of a
	// full block, so the result is exactly one block.
	if err := w.encrypt(PKCS7Pad(w.pending, aes.BlockSize)); err != nil {
		return err
	}
	w.pending = nil
	return nil
}

// contentEncryptor produces the [IV][ciphertext] portion of an .intunewin
// payload as content is written to it, along with the HMAC over that span.
//
// The MAC precedes the IV in the finished blob but is only known once the last
// byte has been encrypted, so callers reserve room for it and fill it in
// afterwards.
type contentEncryptor struct {
	cbc *cbcEncryptWriter
	mac hash.Hash
}

func newContentEncryptor(dst io.Writer, encKey, macKey, iv []byte) (*contentEncryptor, error) {
	if len(encKey) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(encKey))
	}
	if len(macKey) != 32 {
		return nil, fmt.Errorf("MAC key must be 32 bytes, got %d", len(macKey))
	}
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("IV must be 16 bytes, got %d", len(iv))
	}

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// The MAC covers IV||ciphertext, and the IV leads the blob, so both go
	// through the same sink.
	mac := hmac.New(sha256.New, macKey)
	sink := io.MultiWriter(dst, mac)
	if _, err := sink.Write(iv); err != nil {
		return nil, fmt.Errorf("failed to write IV: %w", err)
	}

	return &contentEncryptor{cbc: newCBCEncryptWriter(sink, block, iv), mac: mac}, nil
}

func (e *contentEncryptor) Write(p []byte) (int, error) { return e.cbc.Write(p) }

// Close flushes the final block and returns the HMAC over IV||ciphertext.
func (e *contentEncryptor) Close() ([]byte, error) {
	if err := e.cbc.Close(); err != nil {
		return nil, err
	}
	return e.mac.Sum(nil), nil
}

// countingWriter records how many bytes pass through it and discards them.
type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// writeIntunewinPackage writes the outer archive: the encrypted payload copied
// from content, then Detection.xml. Both entries are stored uncompressed, which
// Intune requires.
func writeIntunewinPackage(w io.Writer, content io.Reader, detectionXML []byte) error {
	zw := zip.NewWriter(w)
	now := time.Now()

	entry := func(name string) (io.Writer, error) {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.Modified = now
		return zw.CreateHeader(header)
	}

	contentWriter, err := entry("IntuneWinPackage/Contents/IntunePackage.intunewin")
	if err != nil {
		return fmt.Errorf("failed to create encrypted content entry: %w", err)
	}
	if _, err := io.Copy(contentWriter, content); err != nil {
		return fmt.Errorf("failed to write encrypted content: %w", err)
	}

	metadataWriter, err := entry("IntuneWinPackage/Metadata/Detection.xml")
	if err != nil {
		return fmt.Errorf("failed to create metadata entry: %w", err)
	}
	if _, err := metadataWriter.Write(detectionXML); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to close package: %w", err)
	}
	return nil
}
