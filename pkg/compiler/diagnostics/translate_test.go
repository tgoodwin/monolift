package diagnostics

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler"
)

func TestTranslateSpanComputesUTF8ByteOffsets(t *testing.T) {
	t.Parallel()

	moduleRoot, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("abs module root: %v", err)
	}
	file := filepath.Join(moduleRoot, "utf8.go")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	expectedStart := bytes.Index(data, []byte("hé"))
	if expectedStart < 0 {
		t.Fatal(`fixture substring "hé" not found`)
	}
	expectedEnd := expectedStart + len([]byte("hé"))

	span, err := TranslateSpan(compiler.Span{
		Filename:  file,
		Line:      3,
		Column:    19,
		EndLine:   3,
		EndColumn: 21,
	}, Options{ModuleRoot: moduleRoot})
	if err != nil {
		t.Fatalf("TranslateSpan: %v", err)
	}

	if span.FileRelativePath != "utf8.go" {
		t.Fatalf("file_relative_path = %q, want utf8.go", span.FileRelativePath)
	}
	if span.ByteOffsetStart != expectedStart || span.ByteOffsetEnd != expectedEnd {
		t.Fatalf("byte offsets = [%d,%d], want [%d,%d]", span.ByteOffsetStart, span.ByteOffsetEnd, expectedStart, expectedEnd)
	}
}

func TestTranslateUnknownCodeReturnsTypedError(t *testing.T) {
	t.Parallel()

	_, err := Translate(compiler.Diagnostic{
		Code:     "MLV2_UNKNOWN_TEST_CODE",
		Severity: compiler.SeverityError,
		Message:  "bad code",
		Span:     compiler.Span{Filename: "unknown", Line: 1, EndLine: 1},
	}, Options{})
	if err == nil {
		t.Fatal("Translate error = nil, want UnknownCodeError")
	}
	var unknown UnknownCodeError
	if !errors.As(err, &unknown) {
		t.Fatalf("Translate error = %T, want UnknownCodeError", err)
	}
	if unknown.Code != "MLV2_UNKNOWN_TEST_CODE" {
		t.Fatalf("unknown code = %q, want MLV2_UNKNOWN_TEST_CODE", unknown.Code)
	}
}

func TestTranslateRoundTripsImplementedCodes(t *testing.T) {
	t.Parallel()

	moduleRoot := t.TempDir()
	file := filepath.Join(moduleRoot, "sample.go")
	if err := os.WriteFile(file, []byte("package sample\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cases := []struct {
		diagnostic compiler.Diagnostic
		ruleIDs    []string
	}{
		{compiler.Diagnostic{Code: compiler.CodeParse, Severity: compiler.SeverityError, Message: "parse", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"PS-ERROR-1"}},
		{compiler.Diagnostic{Code: compiler.CodeUnknownKey, Severity: compiler.SeverityError, Message: "unknown key", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"PS-GRAMMAR-2"}},
		{compiler.Diagnostic{Code: compiler.CodeInvalidKeyForSurface, Severity: compiler.SeverityError, Message: "invalid key", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"PS-KEY-1"}},
		{compiler.Diagnostic{Code: compiler.CodeMisattached, Severity: compiler.SeverityError, Message: "misattached", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"PS-ATTACH-1"}},
		{compiler.Diagnostic{Code: compiler.CodeDuplicate, Severity: compiler.SeverityError, Message: "duplicate", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"PS-ATTACH-2"}},
		{compiler.Diagnostic{Code: compiler.CodeUnknownVerb, Severity: compiler.SeverityError, Message: "verb", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"PS-ATTACH-3"}},
		{compiler.Diagnostic{Code: compiler.CodeV1Deprecated, Severity: compiler.SeverityWarning, Message: "deprecated", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"PS-MIGRATE-2"}},
		{compiler.Diagnostic{Code: "MLV2_REFLECTION_DISPATCH", Severity: compiler.SeverityError, Message: "reflection", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"EC-TERM-4"}},
		{compiler.Diagnostic{Code: "MLV2_DYNAMIC_PLUGIN", Severity: compiler.SeverityError, Message: "plugin", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"EC-TERM-6"}},
		{compiler.Diagnostic{Code: "MLV2_CLOSURE_UNBOUNDED", Severity: compiler.SeverityError, Message: "unbounded", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"EC-TERM-7"}},
		{compiler.Diagnostic{Code: "MLV2_CLOSURE_TOO_LARGE", Severity: compiler.SeverityError, Message: "too large", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"EC-PRUNE-3"}},
		{compiler.Diagnostic{Code: "MLV2_SHAPE_UNSUPPORTED", Severity: compiler.SeverityError, Message: "shape unsupported", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"TA-SHAPE-1", "TA-REFUSE-1", "AS-FUNC-2"}},
		{compiler.Diagnostic{Code: "MLV2_STRUCT_SURFACE_UNSUPPORTED", Severity: compiler.SeverityError, Message: "struct surface unsupported", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"AS-STRUCT-2"}},
		{compiler.Diagnostic{Code: "MLV2_BUILDER_CHAIN_ROOT", Severity: compiler.SeverityError, Message: "builder chain", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"TA-SHAPE-1"}},
		{compiler.Diagnostic{Code: "MLV2_NO_ERROR_CHANNEL", Severity: compiler.SeverityError, Message: "no error channel", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"TA-SHAPE-1", "SS-WALDO-2"}},
		{compiler.Diagnostic{Code: "MLV2_TRANSPORT_RESERVED", Severity: compiler.SeverityError, Message: "transport reserved", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"TA-GRPC-1"}},
		{compiler.Diagnostic{Code: "MLV2_STATE_DECL_CONFLICT", Severity: compiler.SeverityError, Message: "state conflict", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"SS-CLASS-3"}},
		{compiler.Diagnostic{Code: "MLV2_STATE_UNKNOWN", Severity: compiler.SeverityError, Message: "state unknown", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"SS-CLASS-4"}},
		{compiler.Diagnostic{Code: "MLV2_SHARED_MUTABLE_STATE", Severity: compiler.SeverityError, Message: "shared mutable", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"SS-DISP-2"}},
		{compiler.Diagnostic{Code: "MLV2_CHANNEL_BOUNDARY", Severity: compiler.SeverityError, Message: "channel boundary", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"SS-LIFT-4", "TA-SER-7"}},
		{compiler.Diagnostic{Code: "MLV2_SESSION_AFFINITY_UNAVAILABLE", Severity: compiler.SeverityError, Message: "affinity unavailable", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"SS-LIFT-6"}},
		{compiler.Diagnostic{Code: "MLV2_EMBEDDED_DB_APP_ROOT", Severity: compiler.SeverityError, Message: "embedded db", Span: compiler.Span{Filename: file, Line: 1, EndLine: 1}}, []string{"SS-LIFT-6", "SS-DISP-2"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.diagnostic.Code, func(t *testing.T) {
			translated, err := Translate(tc.diagnostic, Options{ModuleRoot: moduleRoot})
			if err != nil {
				t.Fatalf("Translate: %v", err)
			}
			if translated.Code != tc.diagnostic.Code {
				t.Fatalf("code = %q, want %q", translated.Code, tc.diagnostic.Code)
			}
			if translated.Severity != string(tc.diagnostic.Severity) {
				t.Fatalf("severity = %q, want %q", translated.Severity, tc.diagnostic.Severity)
			}
			if translated.Message != tc.diagnostic.Message {
				t.Fatalf("message = %q, want %q", translated.Message, tc.diagnostic.Message)
			}
			if !reflect.DeepEqual(translated.RuleIDs, tc.ruleIDs) {
				t.Fatalf("ruleIds = %v, want %v", translated.RuleIDs, tc.ruleIDs)
			}
			if translated.Remediation == nil || *translated.Remediation == "" {
				t.Fatalf("remediation = %v, want non-empty remediation", translated.Remediation)
			}
			if translated.Span.FileRelativePath != "sample.go" {
				t.Fatalf("file_relative_path = %q, want sample.go", translated.Span.FileRelativePath)
			}
		})
	}
}

func TestTranslatePrefersDiagnosticRuleIDsOverride(t *testing.T) {
	t.Parallel()

	moduleRoot := t.TempDir()
	file := filepath.Join(moduleRoot, "sample.go")
	if err := os.WriteFile(file, []byte("package sample\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	translated, err := Translate(compiler.Diagnostic{
		Code:     "MLV2_SHAPE_UNSUPPORTED",
		Severity: compiler.SeverityError,
		Message:  "handler mismatch",
		Span:     compiler.Span{Filename: file, Line: 1, EndLine: 1},
		RuleIDs:  []string{"TA-HANDLER-1"},
	}, Options{ModuleRoot: moduleRoot})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if !reflect.DeepEqual(translated.RuleIDs, []string{"TA-HANDLER-1"}) {
		t.Fatalf("ruleIds = %v, want [TA-HANDLER-1]", translated.RuleIDs)
	}
}
