package codegen

import (
	"strings"
	"testing"
)

// dischargeLocalLifecycle only consults len(inputs) for an early return; the
// real work iterates fn.Params. A one-element slice is enough to bypass the
// "no adapter inputs" short-circuit.
var lifecycleInputsStub = []AdapterPattern{{}}

// TestDischargeLocalLifecycleRefusesInterfaceBoxing covers the *ssa.MakeInterface
// check (SPRINT-0052 task 2.5 / B-13): boxing an awkward adapter input into an
// interface lets the value escape the helper, so its lifecycle cannot move to
// the remote side.
func TestDischargeLocalLifecycleRefusesInterfaceBoxing(t *testing.T) {
	src := `
package main

import "mime/multipart"

func sink(v any) {}

func helper(file *multipart.FileHeader) error {
	sink(file)
	return nil
}

func main() { _ = helper }
`
	prog := loadAdapterSSA(t, src)
	fn := findSSAFunction(t, prog, "helper")
	proof := dischargeLocalLifecycle(fn, lifecycleInputsStub)
	if proof.Satisfied {
		t.Fatal("expected refusal: boxing the param into an interface should fail adapter_local_lifecycle")
	}
	if !strings.Contains(proof.Detail, "boxed into an interface") {
		t.Fatalf("unexpected detail: %q", proof.Detail)
	}
}

// TestDischargeLocalLifecycleRefusesGlobalStore covers the *ssa.Store-to-global
// check: storing the awkward input into a package-level global escapes the
// helper entirely.
func TestDischargeLocalLifecycleRefusesGlobalStore(t *testing.T) {
	src := `
package main

import "mime/multipart"

var leaked *multipart.FileHeader

func helper(file *multipart.FileHeader) error {
	leaked = file
	return nil
}

func main() { _ = helper }
`
	prog := loadAdapterSSA(t, src)
	fn := findSSAFunction(t, prog, "helper")
	proof := dischargeLocalLifecycle(fn, lifecycleInputsStub)
	if proof.Satisfied {
		t.Fatal("expected refusal: storing the param into a package-level global should fail adapter_local_lifecycle")
	}
	if !strings.Contains(proof.Detail, "package-level global") {
		t.Fatalf("unexpected detail: %q", proof.Detail)
	}
}

// TestDischargeLocalLifecycleAcceptsLocalUse confirms the strengthened checks
// do not refuse the legitimate shape: the param is opened (a concrete method
// call) and only the opened file is closed/read, never the param itself.
func TestDischargeLocalLifecycleAcceptsLocalUse(t *testing.T) {
	src := `
package main

import (
	"io"
	"mime/multipart"
)

func helper(file *multipart.FileHeader) ([]byte, error) {
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func main() { _ = helper }
`
	prog := loadAdapterSSA(t, src)
	fn := findSSAFunction(t, prog, "helper")
	proof := dischargeLocalLifecycle(fn, lifecycleInputsStub)
	if !proof.Satisfied {
		t.Fatalf("expected acceptance for local-only use, got refusal: %q", proof.Detail)
	}
}
