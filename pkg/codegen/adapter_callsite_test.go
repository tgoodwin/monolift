package codegen

import (
	"testing"

	"golang.org/x/tools/go/ssa"
)

// TestBuildCallSiteIndexClassification feeds synthetic programs through the
// reverse-import scan and asserts how each kind of reference to the helper is
// classified: a direct static call is recorded; any function-value use
// (assigned to a package-level var, passed as an argument, reflected) is a
// disqualifier. SPRINT-0052 task 2.4 (flag B-12).
func TestBuildCallSiteIndexClassification(t *testing.T) {
	cases := []struct {
		name            string
		src             string
		wantDirectCalls bool
		wantDisqualify  bool
	}{
		{
			name: "direct call accepted",
			src: `
package main

func helper(x int) int { return x + 1 }

func main() { println(helper(41)) }
`,
			wantDirectCalls: true,
			wantDisqualify:  false,
		},
		{
			name: "assigned to a function variable refused",
			src: `
package main

func helper(x int) int { return x + 1 }

var sink func(int) int

func main() {
	sink = helper
	println(sink(1))
}
`,
			wantDisqualify: true,
		},
		{
			name: "passed as a function value refused",
			src: `
package main

func helper(x int) int { return x + 1 }

func register(fn func(int) int) { println(fn(2)) }

func main() { register(helper) }
`,
			wantDisqualify: true,
		},
		{
			name: "reflective use refused",
			src: `
package main

import "reflect"

func helper(x int) int { return x + 1 }

func main() { _ = reflect.ValueOf(helper) }
`,
			wantDisqualify: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := loadAdapterSSA(t, tc.src)
			fn := findSSAFunction(t, prog, "helper")
			idx := buildCallSiteIndex(fn)
			if !idx.Scanned {
				t.Fatal("index not marked scanned")
			}
			if got := len(idx.DirectCalls) > 0; got != tc.wantDirectCalls {
				t.Errorf("DirectCalls present = %v, want %v (calls=%d)", got, tc.wantDirectCalls, len(idx.DirectCalls))
			}
			if got := idx.Disqualifier != ""; got != tc.wantDisqualify {
				t.Errorf("Disqualifier present = %v (%q), want %v", got, idx.Disqualifier, tc.wantDisqualify)
			}
		})
	}
}

func TestDischargeCallSiteWithIndex(t *testing.T) {
	cases := []struct {
		name          string
		index         *CallSiteIndex
		exported      bool
		wantSatisfied bool
	}{
		{
			name:          "disqualifier refuses",
			index:         &CallSiteIndex{Scanned: true, Disqualifier: "helper is stored as a function value"},
			wantSatisfied: false,
		},
		{
			name:          "direct calls pass",
			index:         &CallSiteIndex{Scanned: true, DirectCalls: []*ssa.CallCommon{{}}},
			wantSatisfied: true,
		},
		{
			name:          "no references, unexported passes",
			index:         &CallSiteIndex{Scanned: true},
			exported:      false,
			wantSatisfied: true,
		},
		{
			name:          "no references, exported refuses",
			index:         &CallSiteIndex{Scanned: true},
			exported:      true,
			wantSatisfied: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proof := dischargeCallSite(AdapterContext{
				CallSiteIndex:    tc.index,
				FunctionExported: tc.exported,
			})
			if proof.Satisfied != tc.wantSatisfied {
				t.Fatalf("Satisfied = %v, want %v (detail=%q)", proof.Satisfied, tc.wantSatisfied, proof.Detail)
			}
		})
	}
}

// TestTryAdapterPassCallSiteScanRefusesFunctionValue is the required
// end-to-end fixture: an otherwise adapter-eligible multipart helper that is
// assigned to a package-level function variable must be refused on the
// adapter_call_site obligation.
func TestTryAdapterPassCallSiteScanRefusesFunctionValue(t *testing.T) {
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

var sink func(*multipart.FileHeader) ([]byte, error)

func main() {
	sink = helper
	_, _ = sink(nil)
}
`
	prog := loadAdapterSSA(t, src)
	fn := findSSAFunction(t, prog, "helper")
	idx := buildCallSiteIndex(fn)
	if idx.Disqualifier == "" {
		t.Fatalf("expected the var-assigned helper to be disqualified, got %+v", idx)
	}
	_, refusals := TryAdapterPass(AdapterContext{Fn: fn, FunctionExported: false, CallSiteIndex: idx})
	if !hasRefusalCode(refusals, RefusalAdapterCallSite) {
		t.Fatalf("expected adapter_call_site refusal, got %+v", refusals)
	}
}

// TestTryAdapterPassCallSiteScanAcceptsDirectCall confirms that a multipart
// helper reached only by direct calls passes the call-site obligation when the
// scan is supplied.
func TestTryAdapterPassCallSiteScanAcceptsDirectCall(t *testing.T) {
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

func main() {
	var fh *multipart.FileHeader
	_, _ = helper(fh)
}
`
	prog := loadAdapterSSA(t, src)
	fn := findSSAFunction(t, prog, "helper")
	idx := buildCallSiteIndex(fn)
	if len(idx.DirectCalls) == 0 || idx.Disqualifier != "" {
		t.Fatalf("expected direct-call-only index, got %+v", idx)
	}
	plan, refusals := TryAdapterPass(AdapterContext{Fn: fn, FunctionExported: false, CallSiteIndex: idx})
	if len(refusals) > 0 {
		t.Fatalf("expected acceptance, got refusals: %+v", refusals)
	}
	requireProofSatisfied(t, plan.Proofs, RefusalAdapterCallSite)
}

func hasRefusalCode(refusals []AdmissionRefusal, code string) bool {
	for _, r := range refusals {
		if r.Code == code {
			return true
		}
	}
	return false
}
