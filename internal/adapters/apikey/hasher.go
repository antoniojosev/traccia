// Package apikey provides Traccia's default APIKeyHasher. Keys are
// high-entropy random tokens, so a fast constant-time hash (SHA-256) is
// used instead of bcrypt/argon2 — those are for low-entropy human
// passwords, not 256-bit random tokens looked up on every track() call.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type SHA256Hasher struct{}

func NewSHA256Hasher() *SHA256Hasher {
	return &SHA256Hasher{}
}

func (h *SHA256Hasher) Generate() (plainKey string, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("apikey: generate: %w", err)
	}
	plainKey = "trc_" + hex.EncodeToString(raw)
	return plainKey, h.Hash(plainKey), nil
}

func (h *SHA256Hasher) Hash(plainKey string) string {
	sum := sha256.Sum256([]byte(plainKey))
	return hex.EncodeToString(sum[:])
}
