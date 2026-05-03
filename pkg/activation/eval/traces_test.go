package eval

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tgoodwin/monolift/pkg/activation"
)

func TestLoadTraceThreeProjects(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		path    string
		project string
		kind    activation.EdgeKind
	}{
		{"docs/research/activation-paths/traces/miniflux-M-1.json", "miniflux", activation.GoroutineLaunch},
		{"docs/research/activation-paths/traces/listmonk-M-1.json", "listmonk", activation.ChannelFlow},
		{"docs/research/activation-paths/traces/pocketbase-M-1.json", "pocketbase", activation.StructFieldFuncValue},
	}

	for _, tc := range cases {
		trace, err := LoadTrace(filepath.Join(root, tc.path))
		if err != nil {
			t.Fatalf("LoadTrace(%s): %v", tc.path, err)
		}
		if trace.Project != tc.project {
			t.Fatalf("project = %q, want %q", trace.Project, tc.project)
		}
		if len(trace.Steps) == 0 {
			t.Fatalf("%s has no steps", trace.ID)
		}
		var found bool
		for _, step := range trace.Steps {
			if step.CanonicalEdgeKind().Kind == tc.kind {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s did not include canonical edge kind %s", trace.ID, tc.kind)
		}
	}
}

func TestLoadTracesEnumeratesCorpus(t *testing.T) {
	traces, err := LoadTraces(filepath.Join(repoRoot(t), "docs/research/activation-paths/traces"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(traces), 72; got != want {
		t.Fatalf("len(traces) = %d, want %d", got, want)
	}
	if traces[0].ID != "caddy/M-1" {
		t.Fatalf("first trace = %s, want caddy/M-1", traces[0].ID)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
}
