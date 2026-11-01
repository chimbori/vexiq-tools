package iqsync

import (
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path with data (FR-3.5). The data goes to a temp file in the same
// directory, which is then renamed over path, so a reader never sees a partial file. The temp
// name ends in a digit, so the watcher ignores it.
func WriteFileAtomic(path string, data []byte) (err error) {
	mode := fs.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
