// Package fsutil has the small file-tree helpers shared by build and publish.
package fsutil

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyTree copies src to dst, keeping symlinks as symlinks.
func CopyTree(src, dst string) error { return CopyTreeSkip(src, dst, nil) }

// CopyTreeSkip is CopyTree, leaving out every path (relative to src) for
// which skip returns true, and anything below it.
func CopyTreeSkip(src, dst string, skip func(rel string) bool) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if skip != nil && rel != "." && skip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			// Owner-writable, whatever the source: dune marks its outputs
			// read-only, and publishing rewrites links in place.
			return copyFile(path, target, info.Mode().Perm()|0o200)
		default:
			return fmt.Errorf("%s: not a regular file, directory or symlink", path)
		}
	})
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
