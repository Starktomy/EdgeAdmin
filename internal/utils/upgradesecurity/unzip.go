package upgradesecurity

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

const MaxExtractedBytes int64 = 2 << 30
const MaxArchiveEntries = 20000

// Extract rejects traversal, links, devices and duplicate entries. All writes
// use an os.Root so even a concurrently introduced symlink cannot escape it.
func Extract(zipPath, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) > MaxArchiveEntries {
		return errors.New("too many archive entries")
	}
	var total uint64
	names := map[string]bool{}
	for _, file := range reader.File {
		name := strings.TrimSuffix(file.Name, "/")
		mode := file.Mode()
		if name == "" || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") || path.Clean(name) != name || (!mode.IsDir() && !mode.IsRegular()) || names[name] {
			return fmt.Errorf("unsafe or duplicate archive entry %q", file.Name)
		}
		names[name] = true
		if file.UncompressedSize64 > uint64(MaxArtifactBytes) || total > uint64(MaxExtractedBytes)-file.UncompressedSize64 {
			return errors.New("archive exceeds extraction size limits")
		}
		total += file.UncompressedSize64
	}
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(targetDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("extraction root must be a real directory")
	}
	root, err := os.OpenRoot(targetDir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, file := range reader.File {
		name := strings.TrimSuffix(file.Name, "/")
		dir := path.Dir(name)
		if file.FileInfo().IsDir() {
			dir = name
		}
		if err := makeDirectories(root, dir); err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if err := extractFile(root, file, name); err != nil {
			return err
		}
	}
	return nil
}

func makeDirectories(root *os.Root, dir string) error {
	if dir == "." {
		return nil
	}
	current := ""
	for _, part := range strings.Split(dir, "/") {
		current = path.Join(current, part)
		if err := root.Mkdir(current, 0700); err != nil && !os.IsExist(err) {
			return err
		}
		info, err := root.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("archive parent must be a real directory")
		}
	}
	return nil
}

func extractFile(root *os.Root, file *zip.File, name string) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	mode := os.FileMode(0600)
	if file.Mode().Perm()&0111 != 0 {
		mode = 0700
	}
	fp, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	n, err := io.Copy(fp, io.LimitReader(reader, int64(file.UncompressedSize64)+1))
	closeErr := fp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n != int64(file.UncompressedSize64) {
		return errors.New("archive entry size differs from its header")
	}
	return nil
}
