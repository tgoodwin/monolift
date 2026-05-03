package eval

import (
	"path/filepath"
	"testing"
)

func TestEnumerateCasesPairsTracesWithTargets(t *testing.T) {
	root := repoRoot(t)
	cases, err := EnumerateCases(
		filepath.Join(root, "docs/research/activation-paths/traces"),
		filepath.Join(root, "evaluation/MANIFEST.yaml"),
		filepath.Join(root, "evaluation"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(cases), 72; got != want {
		t.Fatalf("len(cases) = %d, want %d", got, want)
	}
	seen := map[string]int{}
	for _, c := range cases {
		if c.Target.Name != c.Trace.Project {
			t.Fatalf("%s paired with target %s", c.Trace.ID, c.Target.Name)
		}
		if c.Target.ProjectDir == "" || c.Target.WorkDir == "" || c.Target.PackagePattern == "" {
			t.Fatalf("%s has incomplete target metadata: %+v", c.Trace.ID, c.Target)
		}
		seen[c.Target.Name]++
	}
	wantCounts := map[string]int{
		"caddy": 6, "gitea": 18, "listmonk": 10, "mattermost": 15, "miniflux": 12, "pocketbase": 11,
	}
	for project, want := range wantCounts {
		if seen[project] != want {
			t.Fatalf("seen[%s] = %d, want %d", project, seen[project], want)
		}
	}
}

func TestLoadManifestPackagePatterns(t *testing.T) {
	root := repoRoot(t)
	manifest, err := LoadManifest(
		filepath.Join(root, "evaluation/MANIFEST.yaml"),
		filepath.Join(root, "evaluation"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.ByName["mattermost"].WorkDir; got != filepath.Join(root, "evaluation/mattermost/server") {
		t.Fatalf("mattermost WorkDir = %s", got)
	}
	if got := manifest.ByName["pocketbase"].PackagePattern; got != "./examples/base" {
		t.Fatalf("pocketbase pattern = %s", got)
	}
}
