package diagnostics

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler"
)

func TestRefusalDiagnosticIndexCoversRegisteredAndPinnedCodes(t *testing.T) {
	t.Parallel()

	index, err := refusalDiagnosticIndex()
	if err != nil {
		t.Fatalf("load refusal diagnostic index: %v", err)
	}

	specCodes := map[string]bool{}
	for _, match := range regexp.MustCompile("`(MLV2_[A-Z0-9_]+)`").FindAllStringSubmatch(index, -1) {
		specCodes[match[1]] = true
	}

	required := map[string]bool{}
	for code := range codeSpecs {
		required[code] = true
	}
	for _, code := range []string{
		compiler.CodeDuplicate,
		compiler.CodeInvalidKeyForSurface,
		compiler.CodeMisattached,
		compiler.CodeParse,
		compiler.CodeUnknownKey,
		compiler.CodeUnknownVerb,
		compiler.CodeV1Deprecated,
	} {
		required[code] = true
	}
	for _, code := range []string{
		"MLV2_CHANNEL_BOUNDARY",
		"MLV2_SESSION_AFFINITY_UNAVAILABLE",
		"MLV2_SHARED_MUTABLE_STATE",
		"MLV2_EMBEDDED_DB_APP_ROOT",
		"MLV2_CLOSURE_TOO_LARGE",
	} {
		required[code] = true
	}

	var missing []string
	for code := range required {
		if !specCodes[code] {
			missing = append(missing, code)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("contract Refusal Diagnostic Index missing codes: %v", missing)
	}
}

func refusalDiagnosticIndex() (string, error) {
	path := filepath.Join("..", "..", "..", "docs", "specs", "monolift-v2-contract.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	text := string(data)
	start := strings.Index(text, "### Refusal Diagnostic Index")
	if start < 0 {
		return "", os.ErrNotExist
	}
	text = text[start:]
	end := strings.Index(text, "Refusal-rules pass result:")
	if end < 0 {
		return "", os.ErrNotExist
	}
	return text[:end], nil
}
