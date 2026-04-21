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
