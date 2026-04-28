package stateclass

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestDetectChannelFieldSeam(t *testing.T) {
	reachable := seamReachability(t, map[string]string{
		"A": "Write",
		"B": "Read",
	})

	seams := DetectSeams(reachable)
	if len(seams) != 1 {
		t.Fatalf("seams=%+v, want one channel seam", seams)
	}
	if seams[0].Type != SeamChannelField || seams[0].Field != "Pipe.ch" {
		t.Fatalf("seam=%+v, want Pipe.ch channel seam", seams[0])
	}
	if !reflect.DeepEqual(seams[0].Writers, []string{"A"}) || !reflect.DeepEqual(seams[0].Readers, []string{"B"}) {
		t.Fatalf("seam=%+v, want writers A readers B", seams[0])
	}
}

func TestDetectChannelFieldSeamSkipsSameRootSets(t *testing.T) {
	reachable := seamReachability(t, map[string]string{
		"A": "Both",
		"B": "Both",
	})

	seams := DetectSeams(reachable)
	if len(seams) != 0 {
		t.Fatalf("seams=%+v, want no inter-root channel seam", seams)
	}
}

func TestDetectRecordedMutexAndAtomicFieldSeams(t *testing.T) {
	reachable := seamReachability(t, map[string]string{
		"A": "LockA",
		"B": "LockB",
	})

	seams := DetectSeams(reachable)
	got := map[SeamType]string{}
	for _, seam := range seams {
		got[seam.Type] = seam.Field
	}
	if got[SeamMutexField] != "Pipe.mu" {
		t.Fatalf("seams=%+v, want Pipe.mu mutex seam", seams)
	}
	if got[SeamAtomicField] != "Pipe.count" {
		t.Fatalf("seams=%+v, want Pipe.count atomic seam", seams)
	}
}

func seamReachability(t *testing.T, rootMethods map[string]string) map[string][]*ssa.Function {
	t.Helper()

	dir := filepath.Join("testdata", "seams")
	var firstPragma extract.Pragma
	first := true
	for rootID, method := range rootMethods {
		pragma := extract.Pragma{
			Name:     rootID,
			Surface:  extract.SurfaceStruct,
			DeclName: "Pipe",
			DeclKind: "struct",
			Options:  map[string]string{"methods": method},
			Span: extract.Span{
				Filename: filepath.Join(dir, "root.go"),
				Line:     8,
				EndLine:  8,
			},
		}
		if first {
			firstPragma = pragma
			first = false
		}
	}
	req := extract.Request{Sources: []string{dir}, Pragmas: []extract.Pragma{firstPragma}}
	loaded, err := extract.LoadModule(req)
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	program, err := extract.BuildProgram(loaded)
	if err != nil {
		t.Fatalf("BuildProgram: %v", err)
	}
	env := &sharedFixtureEnv{
		loaded:    loaded,
		program:   program,
		functions: ssautil.AllFunctions(program),
		callGraph: extract.CallGraphForProgram(program),
	}

	out := map[string][]*ssa.Function{}
	for rootID, method := range rootMethods {
		reqForRoot := extract.Request{
			Sources: []string{dir},
			Pragmas: []extract.Pragma{{
				Name:     rootID,
				Surface:  extract.SurfaceStruct,
				DeclName: "Pipe",
				DeclKind: "struct",
				Options:  map[string]string{"methods": method},
				Span: extract.Span{
					Filename: filepath.Join(dir, "root.go"),
					Line:     8,
					EndLine:  8,
				},
			}},
		}
		rootLoaded, err := extract.RebindLoadedModule(loaded, reqForRoot)
		if err != nil {
			t.Fatalf("RebindLoadedModule: %v", err)
		}
		root := extract.ResolveRoot(rootLoaded)
		out[rootID] = reachableFunctionsForRoot(rootLoaded, env, root)
	}
	return out
}
