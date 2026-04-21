package compiler

import "testing"

func requireDiagnostic(t *testing.T, diagnostics []Diagnostic, code string, severity Severity) Diagnostic {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code && diagnostic.Severity == severity {
			return diagnostic
		}
	}
	t.Fatalf("missing diagnostic code=%s severity=%s in %#v", code, severity, diagnostics)
	return Diagnostic{}
}

func requireSpanLineInRange(t *testing.T, diagnostic Diagnostic, minLine, maxLine int) {
	t.Helper()
	if diagnostic.Span.Line < minLine || diagnostic.Span.Line > maxLine {
		t.Fatalf("diagnostic %s line %d outside range [%d,%d]", diagnostic.Code, diagnostic.Span.Line, minLine, maxLine)
	}
	if diagnostic.Span.Column <= 0 || diagnostic.Span.EndColumn < diagnostic.Span.Column {
		t.Fatalf("diagnostic %s has unusable column range: %#v", diagnostic.Code, diagnostic.Span)
	}
}
