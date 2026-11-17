// Package mail implements mail ingest: credential encryption, the template
// extractor that turns a bank alert into a transaction draft, the IMAP source,
// and the poll loop that decides each message's disposition.
package mail

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"

	"golang.org/x/crypto/chacha20poly1305"

	apierrors "github.com/mohan9182/financer/internal/errors"
)

// Encrypt seals plaintext with ChaCha20-Poly1305 under the 32-byte hex key.
// The blob is standard-base64 of nonce||ciphertext||tag, byte-compatible with
// the Rust backend's `crypto.rs`.
func Encrypt(keyHex, plaintext string) (string, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return "", err
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return "", apierrors.BadRequest("FINANCER_SECRET_KEY is not a valid key")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", apierrors.Internal("could not generate a nonce")
	}
	blob := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(blob), nil
}

// Decrypt opens a stored credential blob.
func Decrypt(keyHex, stored string) (string, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return "", err
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return "", apierrors.BadRequest("FINANCER_SECRET_KEY is not a valid key")
	}
	blob, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		return "", apierrors.Internal("stored credential is not valid base64")
	}
	if len(blob) <= aead.NonceSize() {
		return "", apierrors.Internal("stored credential is truncated")
	}
	nonce, ciphertext := blob[:aead.NonceSize()], blob[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", apierrors.BadRequest("could not decrypt the stored credential (wrong FINANCER_SECRET_KEY?)")
	}
	return string(plaintext), nil
}

func decodeKey(keyHex string) ([]byte, error) {
	if len(keyHex) != 64 {
		return nil, apierrors.BadRequest("FINANCER_SECRET_KEY must be 64 hex characters")
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, apierrors.BadRequest("FINANCER_SECRET_KEY is not valid hex")
	}
	return key, nil
}
