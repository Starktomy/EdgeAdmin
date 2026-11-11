package utils

import (
	"encoding/hex"
	"testing"
)

func TestSecurityRandomTokensPreserveFormatAndDoNotRepeat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1024; i++ {
		token, err := RandomToken()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := hex.DecodeString(token)
		if err != nil || len(token) != 32 || len(decoded) != 16 || seen[token] {
			t.Fatal("invalid or repeated security token")
		}
		seen[token] = true
	}
}
