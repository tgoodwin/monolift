//go:build e2e

package harness

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestClusterEnsureLoadTeardown(t *testing.T) {
	if !E2EEnabled() {
		t.Skip("MONOLIFT_E2E=1 required")
	}
	if os.Getenv("MONOLIFT_E2E_CLUSTER_TEST") != "1" {
		t.Skip("MONOLIFT_E2E_CLUSTER_TEST=1 required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cluster := Cluster{
		Name:       "monolift-e2e-cluster-test",
		ConfigPath: "test/e2e/fixtures/kind-config.yaml",
		Logf:       t.Logf,
	}
	defer func() {
		if result, err := RunCommand(context.Background(), "kind", "delete", "cluster", "--name", cluster.Name); err != nil {
			t.Logf("teardown failed: %v: %s", err, TailLines(result.Stderr, 20))
		}
	}()

	if err := cluster.Ensure(ctx); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	tag := "monolift-e2e/tiny:cluster-test"
	if result, err := RunCommand(ctx, "docker", "build", "-f", FromRepoRoot("test/e2e/harness/testdata/tiny.Dockerfile"), "-t", tag, FromRepoRoot("test/e2e/harness/testdata")); err != nil {
		t.Fatalf("docker build tiny image: %v: %s", err, TailLines(result.Stderr+"\n"+result.Stdout, 20))
	}
	if err := cluster.LoadImage(ctx, tag); err != nil {
		t.Fatalf("LoadImage: %v", err)
	}
}
