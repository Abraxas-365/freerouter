package providerkeyinfra

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/Abraxas-365/freerouter/internal/providerkey"
	"golang.org/x/crypto/nacl/secretbox"
)

// Encryptor implements providerkey.TokenEncryptor using NaCl secretbox.
type Encryptor struct {
	key     [32]byte
	hmacKey []byte
}

var _ providerkey.TokenEncryptor = (*Encryptor)(nil)

// NewEncryptor creates an encryptor from a 32-byte hex-encoded key.
func NewEncryptor(hexKey string) (*Encryptor, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(keyBytes))
	}

	var key [32]byte
	copy(key[:], keyBytes)

	h := sha256.Sum256(append(keyBytes, []byte("hmac-key-derivation")...))
	return &Encryptor{key: key, hmacKey: h[:]}, nil
}

func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	sealed := secretbox.Seal(nonce[:], []byte(plaintext), &nonce, &e.key)
	return hex.EncodeToString(sealed), nil
}

func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext encoding: %w", err)
	}
	if len(data) < 24 {
		return "", fmt.Errorf("ciphertext too short")
	}

	var nonce [24]byte
	copy(nonce[:], data[:24])

	plaintext, ok := secretbox.Open(nil, data[24:], &nonce, &e.key)
	if !ok {
		return "", fmt.Errorf("decryption failed")
	}
	return string(plaintext), nil
}

func (e *Encryptor) Mask(plaintext string) string {
	if len(plaintext) <= 8 {
		return strings.Repeat("*", len(plaintext))
	}
	return plaintext[:4] + strings.Repeat("*", len(plaintext)-8) + plaintext[len(plaintext)-4:]
}

func (e *Encryptor) Hash(plaintext string) string {
	mac := hmac.New(sha256.New, e.hmacKey)
	mac.Write([]byte(plaintext))
	return hex.EncodeToString(mac.Sum(nil))
}
