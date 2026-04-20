package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestStubCompilerFixturesValidate(t *testing.T) {
	for _, target := range []string{"caddy", "pocketbase", "miniflux"} {
		t.Run(target, func(t *testing.T) {
			out := t.TempDir()
			cmd := exec.Command("go", "run", ".", "--target="+target, "--output="+out)
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
		})
	}
}
