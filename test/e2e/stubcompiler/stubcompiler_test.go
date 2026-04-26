package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestStubCompilerFixturesValidate(t *testing.T) {
	for _, target := range []string{"caddy", "pocketbase", "miniflux"} {
		t.Run(target, func(t *testing.T) {
			if target == "caddy" && testing.Short() {
				t.Skip("SSA-heavy; load real evaluation corpus")
			}
			out := t.TempDir()
			args := []string{"run", ".", "--target=" + target, "--output=" + out}
			switch target {
			case "caddy":
				args = append(args, "--source=../../../evaluation/caddy")
			case "pocketbase":
				args = append(args, "--source=../../../evaluation/pocketbase")
			}
			cmd := exec.Command("go", args...)
			data, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("stubcompiler failed: %v\n%s", err, data)
			}
			reportData, err := os.ReadFile(filepath.Join(out, "closure-report.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := reportv2.Validate(reportData); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if target == "caddy" {
				if _, err := os.Stat(filepath.Join(out, "lifted", "deployment.yaml")); err != nil {
					t.Fatalf("expected lifted deployment artifact: %v", err)
				}
			}
		})
	}
}

func TestCaddySourceTreeUntouched(t *testing.T) {
	before := hashTree(t, filepath.Join(repoRoot(), "evaluation", "caddy"))
	out := t.TempDir()
	cmd := exec.Command("go", "run", ".", "--target=caddy", "--output="+out, "--source=../../../evaluation/caddy")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("stubcompiler failed: %v\n%s", err, data)
	}
	after := hashTree(t, filepath.Join(repoRoot(), "evaluation", "caddy"))
	if before != after {
		t.Fatalf("evaluation/caddy hash changed: before=%s after=%s", before, after)
	}
}

func hashTree(t *testing.T, root string) string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	sum := sha256.New()
	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		sum.Write([]byte(filepath.ToSlash(rel)))
		sum.Write([]byte{0})
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(sum, file); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}
