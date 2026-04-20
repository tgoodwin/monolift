package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ImageBuilder struct {
	Cluster    Cluster
	Target     string
	SourceDirs []string
	CacheDir   string
}

func (b ImageBuilder) Build(ctx context.Context, dockerfile, contextDir, tag string) error {
	hash, err := b.sourceHash(dockerfile, contextDir)
	if err != nil {
		return StageError(5, b.Target, KindArtifact, "hash source failed: %v", err)
	}
	cacheFile := filepath.Join(b.cacheDir(), safeFilename(tag)+".sha256")
	if cached, err := os.ReadFile(cacheFile); err == nil && strings.TrimSpace(string(cached)) == hash {
		return nil
	}

	args := []string{"build", "-f", FromRepoRoot(dockerfile), "-t", tag, FromRepoRoot(contextDir)}
	result, err := RunCommand(ctx, "docker", args...)
	if err != nil {
		return StageError(5, b.Target, KindArtifact, "docker build failed: %s", TailLines(result.Stderr+"\n"+result.Stdout, 20))
	}
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cacheFile, []byte(hash+"\n"), 0o644)
}

func (b ImageBuilder) LoadToKind(ctx context.Context, tag string) error {
	cluster := b.Cluster
	if cluster.Name == "" {
		cluster = NewCluster()
	}
	if err := cluster.LoadImage(ctx, tag); err != nil {
		return StageError(6, b.Target, KindArtifact, "kind load failed for image %s: %v", tag, err)
	}
	return nil
}

func (b ImageBuilder) sourceHash(dockerfile, contextDir string) (string, error) {
	hasher := sha256.New()
	files := []string{FromRepoRoot(dockerfile)}
	roots := b.SourceDirs
	if len(roots) == 0 {
		roots = []string{contextDir}
	}
	for _, sourceDir := range roots {
		root := FromRepoRoot(sourceDir)
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if entry.IsDir() {
				if entry.Name() == "bin" || entry.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			files = append(files, path)
			return nil
		}); err != nil {
			return "", err
		}
	}
	sort.Strings(files)
	for _, file := range files {
		if err := hashFile(hasher, file); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (b ImageBuilder) cacheDir() string {
	if b.CacheDir != "" {
		return b.CacheDir
	}
	return filepath.Join(os.TempDir(), "monolift-e2e", ".cache")
}

func hashFile(w io.Writer, path string) error {
	if _, err := fmt.Fprintln(w, path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func safeFilename(s string) string {
	replacer := strings.NewReplacer("/", "_", ":", "_", "@", "_")
	return replacer.Replace(s)
}
