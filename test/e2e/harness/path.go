package harness

import (
	"os"
	"path/filepath"
)

func RepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
	}
}

func FromRepoRoot(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(RepoRoot(), path)
}
