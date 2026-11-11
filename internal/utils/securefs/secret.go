// Package securefs provides private, atomic writes for credentials and keys.
package securefs

import (
	"errors"
	"os"
	"path/filepath"
)

func PrivateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("private directory must not be a symlink")
	}
	return os.Chmod(path, 0700)
}

// WriteFile never truncates an existing credential through a symbolic link.
// Creating and renaming a 0600 temporary file also tightens old file modes.
func WriteFile(path string, data []byte) error {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return errors.New("secret target must be a regular file")
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(path)
	parentInfo, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	if !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("secret directory must not be a symlink")
	}
	fp, err := os.CreateTemp(parent, ".edge-secret-*")
	if err != nil {
		return err
	}
	defer os.Remove(fp.Name())
	if _, err = fp.Write(data); err == nil {
		err = fp.Sync()
	}
	closeErr := fp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(fp.Name(), path)
}
