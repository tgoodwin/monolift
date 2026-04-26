package liftability

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/ssa"
)

var (
	fixtureOnce    sync.Once
	fixtureCtx     *Context
	fixtureProgram *ssa.Program
	fixtureLoaded  *extract.LoadedModule
	fixtureErr     error
)

func loadFixtureEnv() (*extract.LoadedModule, *ssa.Program, *Context, error) {
	fixtureOnce.Do(func() {
		rootFile := filepath.Join("testdata", "fixtures", "root.go")
		req := extract.Request{
			Sources: []string{filepath.Join("testdata", "fixtures")},
			Pragmas: []extract.Pragma{{
				Name:     "ContextFirst",
				Surface:  extract.SurfaceFunction,
				DeclName: "ContextFirst",
				DeclKind: "func",
				Options:  map[string]string{"name": "ContextFirst"},
				Span: extract.Span{
					Filename: rootFile,
					Line:     1,
					EndLine:  1,
				},
			}},
		}
		fixtureLoaded, fixtureErr = extract.LoadModule(req)
		if fixtureErr != nil {
			return
		}
		fixtureProgram, fixtureErr = extract.BuildProgram(fixtureLoaded)
		if fixtureErr != nil {
			return
		}
		fixtureCtx, fixtureErr = NewContext(fixtureLoaded, fixtureProgram)
	})
	return fixtureLoaded, fixtureProgram, fixtureCtx, fixtureErr
}

func testContextAndOperation(t *testing.T, decl string) (*Context, Operation) {
	t.Helper()
	loaded, program, ctx, err := loadFixtureEnv()
	if err != nil {
		t.Fatalf("loadFixtureEnv: %v", err)
	}
	op, err := resolveOperation(loaded, program, reportv2.SymbolIdentity{
		ModulePath:  "example.com/liftabilityfixtures",
		PackagePath: "example.com/liftabilityfixtures",
		ObjectName:  decl,
		Kind:        "function",
	})
	if err != nil {
		t.Fatalf("resolveOperation(%s): %v", decl, err)
	}
	return ctx, op
}

func testMethodContextAndOperation(t *testing.T, objectName string) (*Context, Operation) {
	t.Helper()
	return testMethodOnType(t, "Service", "ReadOnly,Mutate", objectName)
}

func testMethodOnType(t *testing.T, declName, methods, objectName string) (*Context, Operation) {
	t.Helper()
	loaded, program, ctx, err := loadFixtureEnv()
	if err != nil {
		t.Fatalf("loadFixtureEnv: %v", err)
	}
	_ = declName
	_ = methods
	op, err := resolveOperation(loaded, program, reportv2.SymbolIdentity{
		ModulePath:  "example.com/liftabilityfixtures",
		PackagePath: "example.com/liftabilityfixtures",
		ObjectName:  objectName,
		Kind:        "method",
	})
	if err != nil {
		t.Fatalf("resolveOperation(%s): %v", objectName, err)
	}
	return ctx, op
}

func assertDetectorVerdict(t *testing.T, detector Detector, ctx *Context, op Operation, want Verdict) []Evidence {
	t.Helper()
	got, evidence, err := detector.Evaluate(ctx, op)
	if err != nil {
		t.Fatalf("%T Evaluate returned error: %v", detector, err)
	}
	if got != want {
		t.Fatalf("%T verdict=%s want %s evidence=%v", detector, got, want, evidence)
	}
	return evidence
}
