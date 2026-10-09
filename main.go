package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func getPath() string {
	baseDir := os.Getenv("XDG_RUNTIME_DIR")
	if baseDir == "" {
		baseDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}

	return filepath.Join(baseDir, "torrent")
}

func main() {

}
