// Package upgradesecurity verifies release identity before installation.
package upgradesecurity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const MaxArtifactBytes int64 = 512 << 20
const MaxManifestBytes int64 = 1 << 20

type Trust struct {
	PublicKeys   map[string]string `json:"publicKeys"`
	AllowedHosts []string          `json:"allowedHosts"`
}

type Manifest struct {
	Component string `json:"component"`
	Version   string `json:"version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
}

type SignedManifest struct {
	KeyID     string `json:"keyId"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

func ParseTrust(data []byte) (*Trust, error) {
	trust := &Trust{}
	if err := json.Unmarshal(data, trust); err != nil {
		return nil, err
	}
	if len(trust.PublicKeys) == 0 || len(trust.AllowedHosts) == 0 {
		return nil, errors.New("upgrade trust requires release public keys and allowed HTTPS hosts")
	}
	for id, value := range trust.PublicKeys {
		key, err := base64.StdEncoding.DecodeString(value)
		if id == "" || err != nil || len(key) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 release public key")
		}
	}
	for _, host := range trust.AllowedHosts {
		if host == "" || strings.ContainsAny(host, "/@?# \\*") {
			return nil, errors.New("invalid allowed upgrade host")
		}
	}
	return trust, nil
}

func (t *Trust) ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return errors.New("upgrade URLs must use HTTPS without credentials or fragments")
	}
	host := strings.ToLower(u.Host)
	for _, allowed := range t.AllowedHosts {
		if host == strings.ToLower(allowed) {
			return nil
		}
	}
	return fmt.Errorf("upgrade host %q is not in upgrade-trust.json", host)
}

func SidecarURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u.Path += ".manifest.json"
	u.RawPath = ""
	return u.String(), nil
}

func Verify(data []byte, trust *Trust, component, osName, arch string) (*Manifest, error) {
	if len(data) > int(MaxManifestBytes) {
		return nil, errors.New("signed release manifest is too large")
	}
	envelope := &SignedManifest{}
	if err := json.Unmarshal(data, envelope); err != nil {
		return nil, fmt.Errorf("invalid signed manifest: %w", err)
	}
	encodedKey, ok := trust.PublicKeys[envelope.KeyID]
	if !ok {
		return nil, errors.New("release manifest is signed by an untrusted key")
	}
	key, keyErr := base64.StdEncoding.DecodeString(encodedKey)
	payload, payloadErr := base64.StdEncoding.DecodeString(envelope.Payload)
	signature, sigErr := base64.StdEncoding.DecodeString(envelope.Signature)
	if keyErr != nil || payloadErr != nil || sigErr != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(ed25519.PublicKey(key), payload, signature) {
		return nil, errors.New("release manifest signature verification failed")
	}
	manifest := &Manifest{}
	if err := json.Unmarshal(payload, manifest); err != nil {
		return nil, err
	}
	hash, err := hex.DecodeString(manifest.SHA256)
	if err != nil || len(hash) != 32 || manifest.Size <= 0 || manifest.Size > MaxArtifactBytes || manifest.Version == "" || manifest.Component != component || manifest.OS != osName || manifest.Arch != arch {
		return nil, errors.New("release manifest does not match the component, platform, digest or size limits")
	}
	return manifest, nil
}
