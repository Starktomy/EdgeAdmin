package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomToken preserves the 32-character format expected by access settings
// while generating 128 bits with the operating system's secure entropy source.
func RandomToken() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
