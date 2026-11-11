package upgradesecurity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestArtifactMatchesSignedDigestAndLength(t *testing.T) {
	digest := sha256.Sum256([]byte("release"))
	manifest := &Manifest{Size: 7, SHA256: hex.EncodeToString(digest[:])}
	for _, content := range []string{"release", "changed", "short", "release-extra"} {
		var dst bytes.Buffer
		err := CopyArtifact(&dst, strings.NewReader(content), manifest)
		if (err == nil) != (content == "release") {
			t.Errorf("artifact %q: %v", content, err)
		}
		if dst.Len() > 8 {
			t.Fatal("untrusted stream exceeded signed size + 1")
		}
	}
}
