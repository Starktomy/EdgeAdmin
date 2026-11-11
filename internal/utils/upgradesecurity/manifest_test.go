package upgradesecurity

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func signedRelease(t *testing.T, manifest Manifest) ([]byte, *Trust) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	envelope := SignedManifest{KeyID: "release", Payload: base64.StdEncoding.EncodeToString(payload), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return data, &Trust{PublicKeys: map[string]string{"release": base64.StdEncoding.EncodeToString(public)}, AllowedHosts: []string{"releases.example"}}
}

func TestManifestRequiresTrustedSignatureAndMatchingPlatform(t *testing.T) {
	hash := sha256.Sum256([]byte("release"))
	manifest := Manifest{Component: "admin", Version: "1.5.0", OS: "linux", Arch: "amd64", SHA256: hex.EncodeToString(hash[:]), Size: 7}
	data, trust := signedRelease(t, manifest)
	if _, err := Verify(data, trust, "admin", "linux", "amd64"); err != nil {
		t.Fatal(err)
	}
	for _, platform := range [][3]string{{"node", "linux", "amd64"}, {"admin", "windows", "amd64"}, {"admin", "linux", "arm64"}} {
		if _, err := Verify(data, trust, platform[0], platform[1], platform[2]); err == nil {
			t.Fatal("accepted wrong component or platform")
		}
	}
	var envelope SignedManifest
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	payload, _ := base64.StdEncoding.DecodeString(envelope.Payload)
	payload = bytes.Replace(payload, []byte("1.5.0"), []byte("9.9.9"), 1)
	envelope.Payload = base64.StdEncoding.EncodeToString(payload)
	tampered, _ := json.Marshal(envelope)
	if _, err := Verify(tampered, trust, "admin", "linux", "amd64"); err == nil {
		t.Fatal("accepted modified signed payload")
	}
	delete(trust.PublicKeys, "release")
	if _, err := Verify(data, trust, "admin", "linux", "amd64"); err == nil {
		t.Fatal("accepted untrusted signer")
	}
}

func TestUpgradeURLRequiresExplicitHTTPSHost(t *testing.T) {
	trust := &Trust{AllowedHosts: []string{"releases.example"}}
	for _, raw := range []string{"http://releases.example/a.zip", "https://evil.example/a.zip", "https://releases.example.evil/a.zip", "https://user:pass@releases.example/a.zip", "https://releases.example/a.zip#fragment", "https://releases.example:8443/a.zip", "file:///tmp/a.zip"} {
		if trust.ValidateURL(raw) == nil {
			t.Errorf("accepted untrusted URL %q", raw)
		}
	}
	if err := trust.ValidateURL("https://releases.example/a.zip"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTrust([]byte(`{"publicKeys":{},"allowedHosts":["releases.example"]}`)); err == nil {
		t.Fatal("accepted unsigned release policy")
	}
}
