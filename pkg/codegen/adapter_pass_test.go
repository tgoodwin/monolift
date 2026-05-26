package codegen

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestAdapterPassNoTargetSpecificCode is the permanent generalization-invariant
// guard for SPRINT-0052: the production code path must carry no knowledge of any
// specific lift target. It scans the target-agnostic framework files for tokens
// that would betray target-specific code and fails if any appears.
//
// The denylist holds genuinely target-specific identifiers — M-4's
// processImage/UploadMedia/listmonk/thumbnailSize/imaging, the 8 MiB magic
// literal (task 1.7 made the limit configurable; only a re-hardcoded
// `8*1024*1024` should trip this), and the selected new targets' function
// names. It deliberately does NOT include `adapter_parent_forbidden`: task 1.8
// (flag A-8) added that to the ADR-0032 refusal vocabulary as a generic,
// target-agnostic structural code, and Phase 1.1 replaced the target-specific
// isUploadMediaCandidate with the structural adapterParentForbiddenForCandidate
// predicate — so the code name is vocabulary, not a fingerprint.
//
// Target #3's identifier is added once Phase 6 locks the pick (the survey's
// provisional candidates are generic words like Send/parse that would
// false-positive, so only specific names go in).
func TestAdapterPassNoTargetSpecificCode(t *testing.T) {
	denylist := regexp.MustCompile(`processImage|UploadMedia|listmonk|countLines|thumbnailSize|disintegration/imaging|8\*1024\*1024`)

	seen := map[string]bool{}
	var files []string
	globbed, err := filepath.Glob("adapter*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range append(globbed, "cut_admit.go", "server.go", "admission.go", "adapter_normalize.go", "adapter_client.go") {
		if strings.HasSuffix(f, "_test.go") || seen[f] {
			continue
		}
		seen[f] = true
		files = append(files, f)
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if denylist.MatchString(line) {
				t.Errorf("%s:%d carries target-specific code: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}

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
