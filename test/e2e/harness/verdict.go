package harness

import (
	"fmt"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type DiagnosticCode string

type Verdict struct{}

func (Verdict) AssertAccept(report *reportv2.Report) error {
	if got := report.Pragma.Options["verdict"]; got != "accept" {
		return fmt.Errorf("verdict=%q want accept", got)
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Severity == "error" {
			return fmt.Errorf("accept report contains refusal diagnostic %s", diagnostic.Code)
		}
	}
	return nil
}

func (Verdict) AssertRefuse(report *reportv2.Report, required []DiagnosticCode) error {
	if got := report.Pragma.Options["verdict"]; got != "refuse-blocking" {
		return fmt.Errorf("verdict=%q want refuse-blocking", got)
	}
	seen := map[DiagnosticCode]bool{}
	for _, diagnostic := range report.Diagnostics {
		seen[DiagnosticCode(diagnostic.Code)] = true
	}
	for _, code := range required {
		if !seen[code] {
			return fmt.Errorf("required diagnostic %s missing", code)
		}
	}
	return nil
}
