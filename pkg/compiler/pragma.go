package compiler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const v2PragmaPrefix = "//monolift:"
const v1AtPrefix = "// " + "@mono" + "lift"
const v1OffloadPrefix = "//monolift:" + "offload"

type Surface string

const (
	SurfaceInterface Surface = "interface"
	SurfaceFunction  Surface = "function"
	SurfaceMethod    Surface = "method"
	SurfaceStruct    Surface = "struct"
	SurfaceUnknown   Surface = "unknown"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

const (
	CodeParse                = "MLV2_PRAGMA_PARSE"
	CodeUnknownKey           = "MLV2_PRAGMA_UNKNOWN_KEY"
	CodeInvalidKeyForSurface = "MLV2_PRAGMA_INVALID_KEY_FOR_SURFACE"
	CodeMisattached          = "MLV2_PRAGMA_MISATTACHED"
	CodeDuplicate            = "MLV2_PRAGMA_DUPLICATE"
	CodeUnknownVerb          = "MLV2_PRAGMA_UNKNOWN_VERB"
	CodeV1Deprecated         = "MLV2_PRAGMA_V1_DEPRECATED"
)

var knownPragmaDiagnosticCodes = []string{
	CodeDuplicate,
	CodeInvalidKeyForSurface,
	CodeMisattached,
	CodeParse,
	CodeUnknownKey,
	CodeUnknownVerb,
	CodeV1Deprecated,
}

type Span struct {
	Filename  string
	Line      int
	Column    int
	EndLine   int
	EndColumn int
}

type Diagnostic struct {
	Code       string
	Severity   Severity
	Message    string
	Span       Span
	RuleIDs    []string
	Suggestion string
}

type Pragma struct {
	Name     string
	Surface  Surface
	Options  map[string]string
	Span     Span
	Raw      string
	DeclName string
	DeclKind string
}

func knownPragmaCodes() []string {
	codes := make([]string, len(knownPragmaDiagnosticCodes))
	copy(codes, knownPragmaDiagnosticCodes)
	return codes
}

func spanFromPosition(base token.Position, startOffset, endOffset int) Span {
	if startOffset < 0 {
		startOffset = 0
	}
	if endOffset < startOffset {
		endOffset = startOffset
	}
	return Span{
		Filename:  base.Filename,
		Line:      base.Line,
		Column:    base.Column + startOffset,
		EndLine:   base.Line,
		EndColumn: base.Column + endOffset,
	}
}

func diagnostic(code string, severity Severity, message string, base token.Position, startOffset, endOffset int) Diagnostic {
	return Diagnostic{
		Code:     code,
		Severity: severity,
		Message:  message,
		Span:     spanFromPosition(base, startOffset, endOffset),
	}
}

func ParseLine(text string, basePos token.Position) (*Pragma, []Diagnostic) {
	if strings.HasPrefix(text, v1AtPrefix) {
		return nil, []Diagnostic{v1Diagnostic(text, basePos, strings.TrimSpace(strings.TrimPrefix(text, v1AtPrefix)))}
	}
	if strings.HasPrefix(text, v1OffloadPrefix) {
		return nil, []Diagnostic{v1Diagnostic(text, basePos, strings.TrimSpace(strings.TrimPrefix(text, v1OffloadPrefix)))}
	}
	if !strings.HasPrefix(text, v2PragmaPrefix) {
		return nil, nil
	}

	verbStart := len(v2PragmaPrefix)
	verbEnd := verbStart
	for verbEnd < len(text) && text[verbEnd] != ' ' && text[verbEnd] != '\t' {
		verbEnd++
	}
	verb := text[verbStart:verbEnd]
	if verb != "lift" {
		return nil, []Diagnostic{diagnostic(CodeUnknownVerb, SeverityError, fmt.Sprintf("unsupported monolift verb %q", verb), basePos, verbStart, verbEnd)}
	}

	i := verbEnd
	options := map[string]string{}
	for {
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		if i >= len(text) {
			break
		}

		keyStart := i
		key, next, err := parseKey(text, i)
		if err != "" {
			return nil, []Diagnostic{diagnostic(CodeParse, SeverityError, err, basePos, keyStart, next)}
		}
		i = next
		if i >= len(text) || text[i] != '=' {
			return nil, []Diagnostic{diagnostic(CodeParse, SeverityError, fmt.Sprintf("option %q is missing '='", key), basePos, keyStart, i)}
		}
		i++
		valueStart := i
		value, next, err := parseValue(text, i)
		if err != "" {
			return nil, []Diagnostic{diagnostic(CodeParse, SeverityError, err, basePos, valueStart, next)}
		}
		i = next
		if previous, ok := options[key]; ok {
			_ = previous
			return nil, []Diagnostic{diagnostic(CodeParse, SeverityError, fmt.Sprintf("duplicate option key %q", key), basePos, keyStart, i)}
		}
		options[key] = value
		if i < len(text) && text[i] != ' ' && text[i] != '\t' {
			return nil, []Diagnostic{diagnostic(CodeParse, SeverityError, "trailing garbage after option value", basePos, i, i+1)}
		}
	}

	return &Pragma{
		Name:    options["name"],
		Options: options,
		Span:    spanFromPosition(basePos, 0, len(text)),
		Raw:     text,
	}, nil
}

func parseKey(text string, start int) (string, int, string) {
	i := start
	for {
		segmentStart := i
		if i >= len(text) || !isASCIILetter(text[i]) {
			return "", i, "empty or invalid option key"
		}
		i++
		for i < len(text) && isIdentContinue(text[i]) {
			i++
		}
		if i < len(text) && (text[i] == '.' || text[i] == ':') {
			i++
			if i >= len(text) {
				return "", i, "option key segment is empty"
			}
			continue
		}
		if segmentStart == i {
			return "", i, "option key segment is empty"
		}
		break
	}
	return text[start:i], i, ""
}

func parseValue(text string, start int) (string, int, string) {
	if start >= len(text) {
		return "", start, "option value is empty"
	}
	if text[start] == '"' {
		var b strings.Builder
		for i := start + 1; i < len(text); i++ {
			switch text[i] {
			case '"':
				return b.String(), i + 1, ""
			case '\n', '\r':
				return "", i, "newline in quoted value"
			case '\\':
				if i+1 >= len(text) {
					return "", i, "unterminated escape in quoted value"
				}
				i++
				switch text[i] {
				case '"':
					b.WriteByte('"')
				case '\\':
					b.WriteByte('\\')
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					return "", i, fmt.Sprintf("invalid escape \\%c", text[i])
				}
			default:
				if text[i] >= 0x80 {
					return "", i, "non-ASCII character in quoted value"
				}
				b.WriteByte(text[i])
			}
		}
		return "", len(text), "unterminated quoted value"
	}

	i := start
	for i < len(text) && isBareChar(text[i]) {
		i++
	}
	if i == start {
		return "", i, "empty or invalid bare value"
	}
	return text[start:i], i, ""
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isIdentContinue(b byte) bool {
	return isASCIILetter(b) || isASCIIDigit(b) || b == '_' || b == '-'
}

func isBareChar(b byte) bool {
	return isASCIILetter(b) || isASCIIDigit(b) || b == '_' || b == '-' || b == '.' || b == '/' || b == ':' || b == ','
}

func FromDecl(decl ast.Decl, fset *token.FileSet) ([]*Pragma, []Diagnostic) {
	if decl == nil {
		return nil, nil
	}
	doc := declDoc(decl)
	if doc == nil {
		return nil, nil
	}

	surface, declName, declKind, surfaceDiag := classifyDecl(decl, fset)
	var pragmas []*Pragma
	var diagnostics []Diagnostic

	for _, comment := range doc.List {
		pragma, diags := ParseLine(comment.Text, fset.Position(comment.Slash))
		diagnostics = append(diagnostics, diags...)
		if pragma == nil {
			continue
		}
		pragma.Surface = surface
		pragma.DeclName = declName
		pragma.DeclKind = declKind
		pragmas = append(pragmas, pragma)
	}

	if len(pragmas) == 0 {
		return nil, diagnostics
	}
	if len(pragmas) > 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:     CodeDuplicate,
			Severity: SeverityError,
			Message:  fmt.Sprintf("multiple monolift:lift pragmas attach to declaration %s", declName),
			Span:     pragmas[1].Span,
		})
	}
	if surfaceDiag != nil {
		diagnostics = append(diagnostics, *surfaceDiag)
		return pragmas, diagnostics
	}
	for _, pragma := range pragmas {
		diagnostics = append(diagnostics, validatePragma(pragma)...)
	}
	return pragmas, diagnostics
}

func Parse(sourceDirs []string) ([]*Pragma, []Diagnostic, error) {
	fset := token.NewFileSet()
	var files []*ast.File
	for _, sourceDir := range sourceDirs {
		err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if shouldSkipSourceDir(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if generated, err := isGeneratedGoFile(path); err == nil && generated {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			files = append(files, file)
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return parseFiles(fset, files)
}

func parseFiles(fset *token.FileSet, files []*ast.File) ([]*Pragma, []Diagnostic, error) {
	var pragmas []*Pragma
	var diagnostics []Diagnostic
	for _, file := range files {
		filePragmas, fileDiagnostics := parseFile(fset, file)
		pragmas = append(pragmas, filePragmas...)
		diagnostics = append(diagnostics, fileDiagnostics...)
	}
	sort.SliceStable(pragmas, func(i, j int) bool {
		if pragmas[i].Span.Filename == pragmas[j].Span.Filename {
			return pragmas[i].Span.Line < pragmas[j].Span.Line
		}
		return pragmas[i].Span.Filename < pragmas[j].Span.Filename
	})
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Span.Filename == diagnostics[j].Span.Filename {
			return diagnostics[i].Span.Line < diagnostics[j].Span.Line
		}
		return diagnostics[i].Span.Filename < diagnostics[j].Span.Filename
	})
	return pragmas, diagnostics, nil
}

func parseFile(fset *token.FileSet, file *ast.File) ([]*Pragma, []Diagnostic) {
	docGroups := map[*ast.CommentGroup]bool{}
	for _, decl := range file.Decls {
		if doc := declDoc(decl); doc != nil {
			docGroups[doc] = true
		}
	}

	var pragmas []*Pragma
	var diagnostics []Diagnostic
	for _, commentGroup := range file.Comments {
		if docGroups[commentGroup] {
			continue
		}
		for _, comment := range commentGroup.List {
			diagnostics = append(diagnostics, diagnosticsForDetachedComment(comment, fset.Position(comment.Slash))...)
		}
	}
	for _, decl := range file.Decls {
		declPragmas, declDiagnostics := FromDecl(decl, fset)
		pragmas = append(pragmas, declPragmas...)
		diagnostics = append(diagnostics, declDiagnostics...)
	}
	return pragmas, diagnostics
}

func diagnosticsForDetachedComment(comment *ast.Comment, base token.Position) []Diagnostic {
	text := comment.Text
	if strings.HasPrefix(text, v1AtPrefix) {
		return []Diagnostic{v1Diagnostic(text, base, strings.TrimSpace(strings.TrimPrefix(text, v1AtPrefix)))}
	}
	if strings.HasPrefix(text, v1OffloadPrefix) {
		return []Diagnostic{v1Diagnostic(text, base, strings.TrimSpace(strings.TrimPrefix(text, v1OffloadPrefix)))}
	}
	if strings.HasPrefix(text, "//monolift:lift") {
		return []Diagnostic{diagnostic(CodeMisattached, SeverityError, "monolift:lift pragma is not attached as a declaration doc comment", base, 0, len(text))}
	}
	if strings.HasPrefix(text, v2PragmaPrefix) {
		_, diags := ParseLine(text, base)
		return diags
	}
	return nil
}

func declDoc(decl ast.Decl) *ast.CommentGroup {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Doc
	case *ast.GenDecl:
		return d.Doc
	default:
		return nil
	}
}

func classifyDecl(decl ast.Decl, fset *token.FileSet) (Surface, string, string, *Diagnostic) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil {
			return SurfaceFunction, d.Name.Name, "func", nil
		}
		return SurfaceMethod, d.Name.Name, "method", nil
	case *ast.GenDecl:
		if d.Tok != token.TYPE {
			return unsupportedDecl(d.Pos(), fset, "unsupported declaration kind "+d.Tok.String())
		}
		if len(d.Specs) != 1 {
			return unsupportedDecl(d.Pos(), fset, "type declaration with multiple specs is unsupported")
		}
		typeSpec, ok := d.Specs[0].(*ast.TypeSpec)
		if !ok {
			return unsupportedDecl(d.Pos(), fset, "unsupported type spec")
		}
		if typeSpec.Assign.IsValid() {
			return unsupportedDecl(typeSpec.Pos(), fset, "type alias declarations are unsupported")
		}
		switch typeSpec.Type.(type) {
		case *ast.InterfaceType:
			return SurfaceInterface, typeSpec.Name.Name, "interface", nil
		case *ast.StructType:
			return SurfaceStruct, typeSpec.Name.Name, "struct", nil
		default:
			return unsupportedDecl(typeSpec.Pos(), fset, fmt.Sprintf("unsupported type declaration %T", typeSpec.Type))
		}
	default:
		return unsupportedDecl(decl.Pos(), fset, fmt.Sprintf("unsupported declaration %T", decl))
	}
}

func unsupportedDecl(pos token.Pos, fset *token.FileSet, message string) (Surface, string, string, *Diagnostic) {
	base := fset.Position(pos)
	diag := diagnostic(CodeParse, SeverityError, message, base, 0, 1)
	return SurfaceUnknown, "", "unsupported", &diag
}

func shouldSkipSourceDir(name string) bool {
	return name == "vendor" || name == "generated" || name == "testdata" || strings.HasPrefix(name, ".")
}

func isGeneratedGoFile(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	prefix := string(content)
	if len(prefix) > 4096 {
		prefix = prefix[:4096]
	}
	return strings.Contains(prefix, "Code generated") && strings.Contains(prefix, "DO NOT EDIT"), nil
}

func v1Diagnostic(text string, basePos token.Position, body string) Diagnostic {
	options := parseV1Options(body)
	metric := options["trigger"]
	if metric == "" {
		metric = options["metric"]
	}
	threshold := options["threshold"]
	suggestion := "//monolift:lift name=<required-by-user> mode=dynamic"
	message := "v1 Monolift pragma syntax is deprecated; rewrite as //monolift:lift and add required v2 keys"
	if metric != "" && threshold != "" {
		suggestion += fmt.Sprintf(" policy=\"trigger=%s threshold=%s\"", metric, threshold)
	} else {
		message += ", including policy=\"trigger=<metric> threshold=<value>\" when dynamic dispatch is intended"
	}
	return Diagnostic{
		Code:       CodeV1Deprecated,
		Severity:   SeverityWarning,
		Message:    message,
		Span:       spanFromPosition(basePos, 0, len(text)),
		Suggestion: suggestion,
	}
}

func parseV1Options(body string) map[string]string {
	options := map[string]string{}
	for _, field := range strings.Fields(body) {
		key, value, ok := strings.Cut(field, "=")
		if !ok || key == "" || value == "" {
			continue
		}
		options[key] = value
	}
	return options
}
