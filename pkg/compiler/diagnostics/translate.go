package diagnostics

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/tgoodwin/monolift/pkg/compiler"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type Options struct {
	ModuleRoot string
}

type UnknownCodeError struct {
	Code string
}

func (e UnknownCodeError) Error() string {
	return fmt.Sprintf("diagnostics: unknown diagnostic code %q", e.Code)
}

type ruleSpec struct {
	RuleIDs            []string
	RemediationBuilder func(compiler.Diagnostic) string
}

func remediationOrDefault(d compiler.Diagnostic, fallback string) string {
	if d.Suggestion != "" {
		return d.Suggestion
	}
	return fallback
}

var codeSpecs = map[string]ruleSpec{
	compiler.CodeParse: {
		RuleIDs: []string{"PS-ERROR-1"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "fix the pragma syntax so it matches the v2 grammar")
		},
	},
	compiler.CodeUnknownKey: {
		RuleIDs: []string{"PS-GRAMMAR-2"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "remove the unknown pragma key or rename it to a supported key")
		},
	},
	compiler.CodeInvalidKeyForSurface: {
		RuleIDs: []string{"PS-KEY-1"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "remove the key or move the pragma to a surface that supports it")
		},
	},
	compiler.CodeMisattached: {
		RuleIDs: []string{"PS-ATTACH-1"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "move the pragma into the declaration doc comment")
		},
	},
	compiler.CodeDuplicate: {
		RuleIDs: []string{"PS-ATTACH-2"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "keep only one monolift:lift pragma on the declaration")
		},
	},
	compiler.CodeUnknownVerb: {
		RuleIDs: []string{"PS-ATTACH-3"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "replace the unsupported monolift verb with monolift:lift")
		},
	},
	compiler.CodeV1Deprecated: {
		RuleIDs: []string{"PS-MIGRATE-2"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "rewrite the pragma using the v2 monolift:lift form")
		},
	},
	"MLV2_REFLECTION_DISPATCH": {
		RuleIDs: []string{"EC-TERM-4"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "replace reflection-driven dispatch with direct calls or an explicit registry-keyed site")
		},
	},
	"MLV2_DYNAMIC_PLUGIN": {
		RuleIDs: []string{"EC-TERM-6"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "replace dynamic plugin resolution with a statically visible registry-keyed implementation set")
		},
	},
	"MLV2_CLOSURE_UNBOUNDED": {
		RuleIDs: []string{"EC-TERM-7"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "remove the unbounded closure edge or make the implementation set finite")
		},
	},
	"MLV2_CLOSURE_TOO_LARGE": {
		RuleIDs: []string{"EC-PRUNE-3"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "lift a narrower function, method, method filter, impl, or registry site")
		},
	},
	"MLV2_SHAPE_UNSUPPORTED": {
		RuleIDs: []string{"TA-SHAPE-1", "TA-REFUSE-1", "AS-FUNC-2"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "change the lifted signature to a supported handler or domain shape")
		},
	},
	"MLV2_STRUCT_SURFACE_UNSUPPORTED": {
		RuleIDs: []string{"AS-STRUCT-2"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "narrow the struct lift with methods= or remove unsupported exported methods from the surface")
		},
	},
	"MLV2_BUILDER_CHAIN_ROOT": {
		RuleIDs: []string{"TA-SHAPE-1"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "lift the terminal request/response entrypoint instead of the fluent builder")
		},
	},
	"MLV2_NO_ERROR_CHANNEL": {
		RuleIDs: []string{"TA-SHAPE-1", "SS-WALDO-2"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "add an error return or keep the root on a framework-managed handler transport")
		},
	},
	"MLV2_TRANSPORT_RESERVED": {
		RuleIDs: []string{"TA-GRPC-1"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "use handler or http-json transport until grpc transport support lands")
		},
	},
	"MLV2_STATE_DECL_CONFLICT": {
		RuleIDs: []string{"SS-CLASS-3"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "change the declared state= value or remove the mutation that contradicts it")
		},
	},
	"MLV2_STATE_UNKNOWN": {
		RuleIDs: []string{"SS-CLASS-4"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "narrow the lifted root or add enough declaration evidence to choose a safe state class")
		},
	},
	"MLV2_SHARED_MUTABLE_STATE": {
		RuleIDs: []string{"SS-DISP-2"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "move shared mutable state behind an external system or narrow the root")
		},
	},
	"MLV2_CHANNEL_BOUNDARY": {
		RuleIDs: []string{"SS-LIFT-4", "TA-SER-7"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "replace the in-process channel boundary with a singleton worker or external queue")
		},
	},
	"MLV2_SESSION_AFFINITY_UNAVAILABLE": {
		RuleIDs: []string{"SS-LIFT-6"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "add a stable affinity key at the lift point or keep the session-bound code local")
		},
	},
	"MLV2_EMBEDDED_DB_APP_ROOT": {
		RuleIDs: []string{"SS-LIFT-6", "SS-DISP-2"},
		RemediationBuilder: func(d compiler.Diagnostic) string {
			return remediationOrDefault(d, "lift a narrower function or externalize database ownership before lifting")
		},
	},
}

func Translate(d compiler.Diagnostic, opts Options) (reportv2.Diagnostic, error) {
	spec, ok := codeSpecs[d.Code]
	if !ok {
		return reportv2.Diagnostic{}, UnknownCodeError{Code: d.Code}
	}
	span, err := translateSpan(d.Span, opts.ModuleRoot)
	if err != nil {
		return reportv2.Diagnostic{}, err
	}
	remediation := spec.RemediationBuilder(d)
	ruleIDs := spec.RuleIDs
	if len(d.RuleIDs) > 0 {
		ruleIDs = d.RuleIDs
	}
	return reportv2.Diagnostic{
		Code:        d.Code,
		Severity:    string(d.Severity),
		Span:        span,
		RuleIDs:     append([]string(nil), ruleIDs...),
		Message:     d.Message,
		Remediation: stringPtr(remediation),
	}, nil
}

func TranslateAll(diags []compiler.Diagnostic, opts Options) ([]reportv2.Diagnostic, error) {
	out := make([]reportv2.Diagnostic, 0, len(diags))
	for _, diagnostic := range diags {
		translated, err := Translate(diagnostic, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, translated)
	}
	return out, nil
}

func TranslateSpan(span compiler.Span, opts Options) (reportv2.SourceSpan, error) {
	return translateSpan(span, opts.ModuleRoot)
}

func translateSpan(span compiler.Span, moduleRoot string) (reportv2.SourceSpan, error) {
	path := span.Filename
	if path == "" {
		path = "unknown"
	}
	fullPath := path
	if moduleRoot != "" && !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(moduleRoot, fullPath)
	}

	relativePath := filepath.ToSlash(path)
	if moduleRoot != "" && filepath.IsAbs(fullPath) {
		if rel, err := filepath.Rel(moduleRoot, fullPath); err == nil {
			relativePath = filepath.ToSlash(rel)
		}
	}

	lineStart := nonZero(span.Line, 1)
	lineEnd := nonZero(span.EndLine, lineStart)
	columnStart := nonZero(span.Column, 1)
	columnEnd := nonZero(span.EndColumn, columnStart)

	byteStart := 0
	byteEnd := 0
	if fullPath != "unknown" {
		offsetStart, offsetEnd, err := byteOffsetsForSpan(fullPath, lineStart, columnStart, lineEnd, columnEnd)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return reportv2.SourceSpan{}, err
		}
		if err == nil {
			byteStart = offsetStart
			byteEnd = offsetEnd
		}
	}

	return reportv2.SourceSpan{
		FileRelativePath: relativePath,
		ByteOffsetStart:  byteStart,
		ByteOffsetEnd:    byteEnd,
		LineStart:        lineStart,
		LineEnd:          lineEnd,
	}, nil
}

func byteOffsetsForSpan(path string, lineStart, columnStart, lineEnd, columnEnd int) (int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}

	lines := splitLines(data)
	start, err := byteOffsetForLocation(lines, lineStart, columnStart)
	if err != nil {
		return 0, 0, err
	}
	end, err := byteOffsetForLocation(lines, lineEnd, columnEnd)
	if err != nil {
		return 0, 0, err
	}
	if end < start {
		end = start
	}
	return start, end, nil
}

func splitLines(data []byte) [][]byte {
	raw := strings.SplitAfter(string(data), "\n")
	lines := make([][]byte, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, []byte(line))
	}
	if len(lines) == 0 {
		return [][]byte{[]byte{}}
	}
	return lines
}

func byteOffsetForLocation(lines [][]byte, line, column int) (int, error) {
	if line < 1 || line > len(lines) {
		return 0, fmt.Errorf("diagnostics: line %d out of range", line)
	}
	offset := 0
	for i := 0; i < line-1; i++ {
		offset += len(lines[i])
	}
	if column <= 1 {
		return offset, nil
	}
	columnOffset, err := byteOffsetForColumn(lines[line-1], column)
	if err != nil {
		return 0, err
	}
	return offset + columnOffset, nil
}

func byteOffsetForColumn(line []byte, column int) (int, error) {
	if column <= 1 {
		return 0, nil
	}
	trimmed := line
	if n := len(trimmed); n > 0 && trimmed[n-1] == '\n' {
		trimmed = trimmed[:n-1]
	}
	currentColumn := 1
	offset := 0
	for len(trimmed) > 0 {
		if currentColumn == column {
			return offset, nil
		}
		_, size := utf8.DecodeRune(trimmed)
		offset += size
		trimmed = trimmed[size:]
		currentColumn++
	}
	if currentColumn == column {
		return offset, nil
	}
	return 0, fmt.Errorf("diagnostics: column %d out of range", column)
}

func nonZero(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
