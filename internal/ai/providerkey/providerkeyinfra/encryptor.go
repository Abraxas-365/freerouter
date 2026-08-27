package providerkeyinfra

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/nacl/secretbox"

	"github.com/Abraxas-365/freerouter/internal/ai/providerkey"
)

// AESTokenEncryptor implements TokenEncryptor using NaCl secretbox
type AESTokenEncryptor struct {
	key     [32]byte
	hmacKey []byte
}

// NewAESTokenEncryptor creates a new encryptor with a 32-byte hex-encoded key
func NewAESTokenEncryptor(hexKey string) (providerkey.TokenEncryptor, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(keyBytes))
	}

	var key [32]byte
	copy(key[:], keyBytes)

	// Derive HMAC key from encryption key
	h := sha256.Sum256(append(keyBytes, []byte("hmac-key-derivation")...))

	return &AESTokenEncryptor{key: key, hmacKey: h[:]}, nil
}

func (e *AESTokenEncryptor) Encrypt(plaintext string) (string, error) {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	sealed := secretbox.Seal(nonce[:], []byte(plaintext), &nonce, &e.key)
	return hex.EncodeToString(sealed), nil
}

func (e *AESTokenEncryptor) Decrypt(ciphertext string) (string, error) {
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

func (e *AESTokenEncryptor) Mask(plaintext string) string {
	if len(plaintext) <= 8 {
		return strings.Repeat("*", len(plaintext))
	}
	return plaintext[:4] + strings.Repeat("*", len(plaintext)-8) + plaintext[len(plaintext)-4:]
}

func (e *AESTokenEncryptor) Hash(plaintext string) string {
	mac := hmac.New(sha256.New, e.hmacKey)
	mac.Write([]byte(plaintext))
	return hex.EncodeToString(mac.Sum(nil))
}
