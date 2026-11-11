package upgradesecurity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

// CopyArtifact bounds disk use and checks the signed length and digest. The
// destination must be private staging storage, never the live installation.
func CopyArtifact(dst io.Writer, src io.Reader, manifest *Manifest) error {
	if manifest == nil || manifest.Size <= 0 || manifest.Size > MaxArtifactBytes {
		return errors.New("invalid artifact size")
	}
	digest := sha256.New()
	size, err := io.Copy(io.MultiWriter(dst, digest), io.LimitReader(src, manifest.Size+1))
	if err != nil {
		return err
	}
	if size != manifest.Size || !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), manifest.SHA256) {
		return errors.New("release artifact digest or size does not match signed manifest")
	}
	return nil
}
