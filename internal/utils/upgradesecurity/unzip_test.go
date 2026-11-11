package upgradesecurity

import (
	"archive/zip"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func makeArchive(t *testing.T, headers ...zip.FileHeader) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "release.zip")
	fp, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(fp)
	for _, header := range headers {
		entry, err := writer.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("release")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := fp.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestExtractRejectsTraversalLinksAndDuplicates(t *testing.T) {
	for _, name := range []string{"../escaped", "/absolute", "safe/../../escaped", "safe\\escaped", "C:/escaped"} {
		t.Run(name, func(t *testing.T) {
			archive := makeArchive(t, zip.FileHeader{Name: name})
			if err := Extract(archive, filepath.Join(t.TempDir(), "out")); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
	link := zip.FileHeader{Name: "link"}
	link.SetMode(os.ModeSymlink | 0777)
	if err := Extract(makeArchive(t, link), filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("archive symlink accepted")
	}
	if err := Extract(makeArchive(t, zip.FileHeader{Name: "same"}, zip.FileHeader{Name: "same"}), filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("duplicate entry accepted")
	}
}

func TestExtractSafeArchiveAndPreexistingLink(t *testing.T) {
	archive := makeArchive(t, zip.FileHeader{Name: "edge-admin/bin/edge-admin"})
	out := filepath.Join(t.TempDir(), "out")
	if err := Extract(archive, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "edge-admin/bin/edge-admin"))
	if err != nil || string(data) != "release" {
		t.Fatal("safe release not extracted", err)
	}
	linkRoot := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(linkRoot, "edge-admin")); err != nil {
		t.Skip(err)
	}
	if err := Extract(archive, linkRoot); err == nil {
		t.Fatal("followed preexisting extraction symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "bin/edge-admin")); !os.IsNotExist(err) {
		t.Fatal("wrote outside extraction root")
	}
}

func TestExtractRejectsOversizedHeadersBeforeWriting(t *testing.T) {
	archive := makeArchive(t, zip.FileHeader{Name: "large"})
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+46 <= len(data); i++ {
		if string(data[i:i+4]) == "PK\x01\x02" {
			binary.LittleEndian.PutUint32(data[i+24:i+28], uint32(MaxArtifactBytes+1))
			break
		}
	}
	if err := os.WriteFile(archive, data, 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	if err := Extract(archive, out); err == nil {
		t.Fatal("oversized archive entry accepted")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("created extraction output before validating archive limits")
	}
}
