package compiler

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestParseLinePositiveGrammar(t *testing.T) {
	base := token.Position{Filename: "fixture.go", Line: 10, Column: 1}
	tests := []struct {
		name string
		line string
		want map[string]string
	}{
		{
			name: "canonical options",
			line: "//monolift:lift name=sender mode=remote state=external transport=http-json",
			want: map[string]string{"name": "sender", "mode": "remote", "state": "external", "transport": "http-json"},
		},
		{
			name: "space and tab separated",
			line: "//monolift:lift\tname=worker\tmode=local state=stateless",
			want: map[string]string{"name": "worker", "mode": "local", "state": "stateless"},
		},
		{
			name: "dotted colon and extension keys",
			line: "//monolift:lift name=svc method:ServeHTTP=remote x-vendor.key=on",
			want: map[string]string{"name": "svc", "method:ServeHTTP": "remote", "x-vendor.key": "on"},
		},
		{
			name: "bare value characters",
			line: "//monolift:lift name=svc registry=tls.issuance/acme:1,A",
			want: map[string]string{"name": "svc", "registry": "tls.issuance/acme:1,A"},
		},
		{
			name: "quoted value with embedded equals",
			line: "//monolift:lift name=svc policy=\"trigger=CPU threshold=0.70\"",
			want: map[string]string{"name": "svc", "policy": "trigger=CPU threshold=0.70"},
		},
		{
			name: "quoted escapes",
			line: "//monolift:lift name=svc policy=\"a\\\"b\\\\c\\n\\t\"",
			want: map[string]string{"name": "svc", "policy": "a\"b\\c\n\t"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pragma, diagnostics := ParseLine(tt.line, base)
			if len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %#v", diagnostics)
			}
			if pragma == nil {
				t.Fatal("expected pragma")
			}
			for key, want := range tt.want {
				if got := pragma.Options[key]; got != want {
					t.Fatalf("option %s got %q want %q", key, got, want)
				}
			}
		})
	}
}

func TestParseLineNegativeGrammar(t *testing.T) {
	base := token.Position{Filename: "fixture.go", Line: 12, Column: 3}
	tests := []struct {
		name string
		line string
		code string
	}{
		{"unknown verb", "//monolift:retire name=svc", CodeUnknownVerb},
		{"valueless flag", "//monolift:lift name=svc async", CodeParse},
		{"empty key", "//monolift:lift =value", CodeParse},
		{"unterminated quote", "//monolift:lift name=svc policy=\"trigger=CPU", CodeParse},
		{"invalid escape", "//monolift:lift name=svc policy=\"bad\\x\"", CodeParse},
		{"missing equals", "//monolift:lift name=svc mode", CodeParse},
		{"non ascii bare value", "//monolift:lift name=café", CodeParse},
		{"trailing garbage", "//monolift:lift name=svc;", CodeParse},
		{"duplicate key", "//monolift:lift name=svc name=other", CodeParse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pragma, diagnostics := ParseLine(tt.line, base)
			if pragma != nil {
				t.Fatalf("unexpected pragma: %#v", pragma)
			}
			diag := requireDiagnostic(t, diagnostics, tt.code, SeverityError)
			requireSpanLineInRange(t, diag, 12, 12)
		})
	}
}

func TestParseLineIgnoresOrdinaryComments(t *testing.T) {
	pragma, diagnostics := ParseLine("// ordinary comment", token.Position{Line: 1, Column: 1})
	if pragma != nil || diagnostics != nil {
		t.Fatalf("ordinary comment got pragma=%#v diagnostics=%#v", pragma, diagnostics)
	}
}

func TestWorkedExamplesParseAndValidate(t *testing.T) {
	examples := []string{
		`package p
import "context"
//monolift:lift name=profile-store mode=dynamic state=external transport=http-json policy="trigger=CPU threshold=0.70"
type ProfileStore interface { Load(context.Context, int64) (*Profile, error) }`,
		`package p
import "context"
//monolift:lift name=campaign-worker mode=remote state=singleton transport=http-json
func worker(ctx context.Context, jobs <-chan CampaignJob) error { return nil }`,
		`package p
import "context"
type UserService struct{}
//monolift:lift name=user-create mode=remote state=external transport=grpc
func (s *UserService) CreateUser(ctx context.Context, u *User) (*User, error) { return u, nil }`,
		`package p
//monolift:lift name=reverse-proxy mode=remote state=external transport=handler methods=ServeHTTP
type ReverseProxy struct{}`,
		`package p
import "context"
//monolift:lift name=mailer mode=remote impl=SMTPSender transport=http-json
type Sender interface { Send(context.Context, Message) error }`,
		`package p
//monolift:lift name=acme-issuer mode=remote state=singleton registry="tls.issuance.acme" methods=Provision,Issue
type ACMEIssuer struct{}`,
		`package p
import "context"
//monolift:lift name=expensive-render mode=local state=stateless
func Render(ctx context.Context, req RenderRequest) (RenderResponse, error) { return RenderResponse{}, nil }`,
	}
	for _, src := range examples {
		t.Run(firstPragmaLine(src), func(t *testing.T) {
			pragmas, diagnostics := parseSource(t, src)
			if len(pragmas) != 1 {
				t.Fatalf("got %d pragmas, want 1", len(pragmas))
			}
			for _, diag := range diagnostics {
				if diag.Severity == SeverityError {
					t.Fatalf("unexpected error diagnostic: %#v", diag)
				}
			}
		})
	}
}

func TestASTAttachmentAndSurfaceClassification(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		surface Surface
	}{
		{"interface", `package p
//monolift:lift name=sender
type Sender interface { Send() }`, SurfaceInterface},
		{"struct", `package p
//monolift:lift name=store
type Store struct{}`, SurfaceStruct},
		{"function", `package p
//monolift:lift name=work
func Work() {}`, SurfaceFunction},
		{"method", `package p
type Store struct{}
//monolift:lift name=run
func (s *Store) Run() {}`, SurfaceMethod},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pragmas, diagnostics := parseSource(t, tt.src)
			if len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %#v", diagnostics)
			}
			if len(pragmas) != 1 || pragmas[0].Surface != tt.surface {
				t.Fatalf("got pragmas %#v want surface %s", pragmas, tt.surface)
			}
		})
	}
}

func TestMisattachedAndDuplicatePragmas(t *testing.T) {
	t.Run("trailing comment", func(t *testing.T) {
		_, diagnostics := parseSource(t, `package p
type Store struct{} //monolift:lift name=store`)
		diag := requireDiagnostic(t, diagnostics, CodeMisattached, SeverityError)
		requireSpanLineInRange(t, diag, 2, 2)
	})
	t.Run("separated comment", func(t *testing.T) {
		_, diagnostics := parseSource(t, `package p
//monolift:lift name=work

func Work() {}`)
		diag := requireDiagnostic(t, diagnostics, CodeMisattached, SeverityError)
		requireSpanLineInRange(t, diag, 2, 2)
	})
	t.Run("duplicate", func(t *testing.T) {
		_, diagnostics := parseSource(t, `package p
//monolift:lift name=one
//monolift:lift name=two
func Work() {}`)
		diag := requireDiagnostic(t, diagnostics, CodeDuplicate, SeverityError)
		requireSpanLineInRange(t, diag, 3, 3)
	})
}

func TestMisattachmentDetectorIgnoresStringLiterals(t *testing.T) {
	_, diagnostics := parseSource(t, `package p
const marker = "monolift:lift name=not-a-comment"`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}

func TestUnsupportedDeclKinds(t *testing.T) {
	_, diagnostics := parseSource(t, `package p
//monolift:lift name=value
var value = func() {}`)
	requireDiagnostic(t, diagnostics, CodeParse, SeverityError)
}

func TestPerSurfaceKeyValidation(t *testing.T) {
	valid := []string{
		`package p
//monolift:lift name=iface mode=remote state=external transport=http-json impl=Impl registry=key methods=A,B dispatch=lift-point affinity=session policy="trigger=CPU threshold=0.70"
type I interface{ A() }`,
		`package p
//monolift:lift name=fn mode=remote state=stateless transport=http-json affinity=session policy="trigger=CPU threshold=0.70"
func Fn() {}`,
		`package p
type S struct{}
//monolift:lift name=m mode=remote state=external transport=http-json affinity=session policy="trigger=CPU threshold=0.70"
func (s *S) M() {}`,
		`package p
//monolift:lift name=s mode=remote state=singleton transport=handler registry=key methods=M affinity=session policy="trigger=CPU threshold=0.70"
type S struct{}`,
	}
	for _, src := range valid {
		t.Run("valid "+firstPragmaLine(src), func(t *testing.T) {
			_, diagnostics := parseSource(t, src)
			for _, diag := range diagnostics {
				if diag.Severity == SeverityError {
					t.Fatalf("unexpected diagnostic: %#v", diag)
				}
			}
		})
	}

	invalids := []struct {
		name string
		src  string
		code string
	}{
		{"unknown key", `package p
//monolift:lift name=fn mystery=value
func Fn() {}`, CodeUnknownKey},
		{"impl on function", `package p
//monolift:lift name=fn impl=Impl
func Fn() {}`, CodeInvalidKeyForSurface},
		{"methods on function", `package p
//monolift:lift name=fn methods=A
func Fn() {}`, CodeInvalidKeyForSurface},
		{"dispatch on method", `package p
type S struct{}
//monolift:lift name=m dispatch=lift-point
func (s *S) M() {}`, CodeInvalidKeyForSurface},
		{"impl on struct", `package p
//monolift:lift name=s impl=Impl
type S struct{}`, CodeInvalidKeyForSurface},
		{"missing name", `package p
//monolift:lift mode=remote
func Fn() {}`, CodeInvalidKeyForSurface},
		{"dynamic missing policy", `package p
//monolift:lift name=fn mode=dynamic
func Fn() {}`, CodeInvalidKeyForSurface},
		{"bad mode", `package p
//monolift:lift name=fn mode=lolwat
func Fn() {}`, CodeParse},
		{"bad state", `package p
//monolift:lift name=fn state=nonsense
func Fn() {}`, CodeParse},
		{"bad transport", `package p
//monolift:lift name=fn transport=pipe
func Fn() {}`, CodeParse},
		{"bad dispatch", `package p
//monolift:lift name=i dispatch=somewhere
type I interface{ M() }`, CodeParse},
	}
	for _, tt := range invalids {
		t.Run(tt.name, func(t *testing.T) {
			_, diagnostics := parseSource(t, tt.src)
			requireDiagnostic(t, diagnostics, tt.code, SeverityError)
		})
	}
}

func TestExtensionKeysAcceptedOnAllSurfaces(t *testing.T) {
	sources := []string{
		`package p
//monolift:lift name=i x-vendor.anything=value x-caddy.mode="custom"
type I interface{ M() }`,
		`package p
//monolift:lift name=f x-vendor.anything=value x-caddy.mode="custom"
func F() {}`,
		`package p
type S struct{}
//monolift:lift name=m x-vendor.anything=value x-caddy.mode="custom"
func (s *S) M() {}`,
		`package p
//monolift:lift name=s x-vendor.anything=value x-caddy.mode="custom"
type S struct{}`,
	}
	for _, src := range sources {
		t.Run(firstPragmaLine(src), func(t *testing.T) {
			pragmas, diagnostics := parseSource(t, src)
			if len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %#v", diagnostics)
			}
			if pragmas[0].Options["x-vendor.anything"] != "value" || pragmas[0].Options["x-caddy.mode"] != "custom" {
				t.Fatalf("extension keys not preserved: %#v", pragmas[0].Options)
			}
		})
	}
}

func TestCanonicalShapeChecksDeferred(t *testing.T) {
	sources := []string{
		`package p
//monolift:lift name=fn mode=local transport=grpc
func Fn() {}`,
		`package p
//monolift:lift name=fn state=affinity
func Fn() {}`,
		`package p
//monolift:lift name=fn transport=handler
func Fn() {}`,
	}
	for _, src := range sources {
		t.Run(firstPragmaLine(src), func(t *testing.T) {
			_, diagnostics := parseSource(t, src)
			for _, diag := range diagnostics {
				if diag.Severity == SeverityError {
					t.Fatalf("parser overreached into canonical-shape validation: %#v", diag)
				}
			}
		})
	}
}

func TestV1MigrationWarnings(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		wantSuggestion string
	}{
		{
			name: "at form trigger",
			src: `package p
// @monolift trigger=CPU threshold=0.70
func Legacy() {}`,
			wantSuggestion: `policy="trigger=CPU threshold=0.70"`,
		},
		{
			name: "offload form metric",
			src: `package p
//monolift:offload metric=MEM threshold=0.80
func Legacy() {}`,
			wantSuggestion: `policy="trigger=MEM threshold=0.80"`,
		},
		{
			name: "missing threshold",
			src: `package p
// @monolift trigger=CPU
func Legacy() {}`,
			wantSuggestion: `//monolift:lift name=<required-by-user> mode=dynamic`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pragmas, diagnostics := parseSource(t, tt.src)
			if len(pragmas) != 0 {
				t.Fatalf("v1 pragma produced accepted v2 pragma: %#v", pragmas)
			}
			diag := requireDiagnostic(t, diagnostics, CodeV1Deprecated, SeverityWarning)
			if !strings.Contains(diag.Suggestion, tt.wantSuggestion) {
				t.Fatalf("suggestion %q does not contain %q", diag.Suggestion, tt.wantSuggestion)
			}
		})
	}
}

func TestMixedV1AndV2OnSameDecl(t *testing.T) {
	pragmas, diagnostics := parseSource(t, `package p
// @monolift trigger=CPU threshold=0.70
//monolift:lift name=modern mode=remote
func Modern() {}`)
	if len(pragmas) != 1 {
		t.Fatalf("got %d pragmas, want 1", len(pragmas))
	}
	requireDiagnostic(t, diagnostics, CodeV1Deprecated, SeverityWarning)
}

func TestRegroupPragmas(t *testing.T) {
	t.Run("same name peers coalesce", func(t *testing.T) {
		pragmas, diagnostics := parseSource(t, `package p
type Hub struct{}
type WebConn struct{}
//monolift:lift name=connection-hub-buffer mode=remote transport=http-json
func (h *Hub) Broadcast() {}
//monolift:lift name=connection-hub-buffer mode=remote transport=http-json
func (wc *WebConn) Pump() {}`)
		if len(diagnostics) != 0 {
			t.Fatalf("unexpected parse diagnostics: %#v", diagnostics)
		}
		regions, diagnostics := RegroupPragmas(pragmas)
		if len(diagnostics) != 0 {
			t.Fatalf("unexpected regroup diagnostics: %#v", diagnostics)
		}
		if len(regions) != 1 || len(regions[0].Roots) != 2 {
			t.Fatalf("regions=%#v, want one region with two roots", regions)
		}
		if regions[0].Roots[0].ID != "Hub.Broadcast" || regions[0].Roots[1].ID != "WebConn.Pump" {
			t.Fatalf("root ids=%q,%q", regions[0].Roots[0].ID, regions[0].Roots[1].ID)
		}
	})

	t.Run("conflicting mode", func(t *testing.T) {
		pragmas, parseDiagnostics := parseSource(t, `package p
//monolift:lift name=svc mode=remote
func A() {}
//monolift:lift name=svc mode=local
func B() {}`)
		if len(parseDiagnostics) != 0 {
			t.Fatalf("unexpected parse diagnostics: %#v", parseDiagnostics)
		}
		_, diagnostics := RegroupPragmas(pragmas)
		requireDiagnostic(t, diagnostics, CodeRegionConflict, SeverityError)
	})

	t.Run("different names", func(t *testing.T) {
		pragmas, diagnostics := parseSource(t, `package p
//monolift:lift name=a
func A() {}
//monolift:lift name=b
func B() {}`)
		if len(diagnostics) != 0 {
			t.Fatalf("unexpected parse diagnostics: %#v", diagnostics)
		}
		regions, diagnostics := RegroupPragmas(pragmas)
		if len(diagnostics) != 0 {
			t.Fatalf("unexpected regroup diagnostics: %#v", diagnostics)
		}
		if len(regions) != 2 {
			t.Fatalf("got %d regions, want 2", len(regions))
		}
	})

	t.Run("empty name legacy fallback", func(t *testing.T) {
		pragmas := []*Pragma{
			{Span: Span{Filename: "a.go", Line: 1}, DeclName: "A", Options: map[string]string{"mode": "remote"}},
			{Span: Span{Filename: "a.go", Line: 2}, DeclName: "B", Options: map[string]string{"mode": "remote"}},
		}
		regions, diagnostics := RegroupPragmas(pragmas)
		if len(diagnostics) != 0 {
			t.Fatalf("unexpected regroup diagnostics: %#v", diagnostics)
		}
		if len(regions) != 2 || len(regions[0].Roots) != 1 || len(regions[1].Roots) != 1 {
			t.Fatalf("regions=%#v, want two single-root regions", regions)
		}
	})

	t.Run("three peers sorted deterministically", func(t *testing.T) {
		pragmas := []*Pragma{
			{Name: "svc", Span: Span{Filename: "a.go", Line: 3}, DeclIdentity: "C", Options: map[string]string{"name": "svc", "mode": "remote"}},
			{Name: "svc", Span: Span{Filename: "a.go", Line: 2}, DeclIdentity: "A", Options: map[string]string{"name": "svc", "mode": "remote"}},
			{Name: "svc", Span: Span{Filename: "a.go", Line: 1}, DeclIdentity: "B", Options: map[string]string{"name": "svc", "mode": "remote"}},
		}
		regions, diagnostics := RegroupPragmas(pragmas)
		if len(diagnostics) != 0 {
			t.Fatalf("unexpected regroup diagnostics: %#v", diagnostics)
		}
		if len(regions) != 1 || len(regions[0].Roots) != 3 {
			t.Fatalf("regions=%#v, want one three-root region", regions)
		}
		got := []string{regions[0].Roots[0].ID, regions[0].Roots[1].ID, regions[0].Roots[2].ID}
		if strings.Join(got, ",") != "A,B,C" {
			t.Fatalf("root order=%v, want A,B,C", got)
		}
	})

	t.Run("post default mode equivalence", func(t *testing.T) {
		pragmas, diagnostics := parseSource(t, `package p
//monolift:lift name=svc
func A() {}
//monolift:lift name=svc mode=remote
func B() {}`)
		if len(diagnostics) != 0 {
			t.Fatalf("unexpected parse diagnostics: %#v", diagnostics)
		}
		_, diagnostics = RegroupPragmas(pragmas)
		if len(diagnostics) != 0 {
			t.Fatalf("unexpected regroup diagnostics: %#v", diagnostics)
		}
	})
}

func parseSource(t *testing.T, src string) ([]*Pragma, []Diagnostic) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "fixture.go", src, goparser.ParseComments)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	pragmas, diagnostics, err := parseFiles(fset, []*ast.File{file})
	if err != nil {
		t.Fatalf("parse files: %v", err)
	}
	return pragmas, diagnostics
}

func firstPragmaLine(src string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//monolift:") || strings.HasPrefix(line, "// @monolift") {
			return line
		}
	}
	return "source"
}
