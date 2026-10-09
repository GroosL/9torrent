package xdg

import (
	"os"
	"fmt"
	"path/filepath"
)

func RuntimeDir() (string, error) {
	path := os.Getenv("XDG_RUNTIME_DIR")
	if path == "" {
		fallback := fmt.Sprintf("/run/user/%d", os.Getuid())
		if info, err := os.Stat(fallback); err == nil && info.IsDir() {
			path = fallback
		} else {
			path = filepath.Join(os.TempDir(), fmt.Sprintf("torrent-%d", os.Getuid()))
		}
	}
	
	path = filepath.Join(path, "torrent")
	
	if err := os.MkdirAll(path, 0700); err != nil {
		return "", fmt.Errorf("create runtime dir: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "",  fmt.Errorf("stat runtime dir: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("%s exists but is not a directory", path)
	}

	perm := info.Mode().Perm()
	if perm != 0700 {
		return "", fmt.Errorf("insecure permissions on %s: got %#o, wanted 0700", path, perm)
	}

	return path, nil
}
