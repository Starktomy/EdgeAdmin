package securefs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSecretWriteTightensModeAndRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(name, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(name, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("secret mode: %v", info.Mode())
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(name, link); err != nil {
		if runtime.GOOS == "windows" {
			return
		}
		t.Fatal(err)
	}
	if err := WriteFile(link, []byte("wrong")); err == nil {
		t.Fatal("followed a symlink")
	}
	data, err := os.ReadFile(name)
	if err != nil || string(data) != "new" {
		t.Fatal("symlink target changed", err)
	}
}
