package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// adapterFuncName returns the exported MonoliftInvoke adapter function name.
// Capitalizes the first letter of the cut function name:
//
//	"funcMarkdown" → "MonoliftInvokeFuncMarkdown"
//	"SanitizeHTML" → "MonoliftInvokeSanitizeHTML"
func adapterFuncName(funcName string) string {
	if funcName == "" {
		return "MonoliftInvoke"
	}
	return "MonoliftInvoke" + strings.ToUpper(funcName[:1]) + funcName[1:]
}

// AdapterFilePath returns the path for the invocation adapter in the cut package.
func AdapterFilePath(plan *Plan) string {
	return filepath.Join(plan.CutPoint.PackageDir, "monolift_adapter_"+plan.EnvServiceName+".go")
}

type adapterView struct {
	PackageName string
	Imports     []importSpec
	AdapterName string
	ParamList   string
	ResultList  string
	HasResults  bool
	CallExpr    string
}

// RenderAdapter generates a same-package exported adapter function that the
// extracted server calls instead of calling the cut symbol directly. For
// methods the adapter flattens the receiver into a regular function parameter.
// For unexported functions/methods the adapter provides an exported entry point.
func RenderAdapter(plan *Plan) (map[string][]byte, error) {
	viewPlan := plan
	if plan != nil && plan.AdapterPlan != nil {
		viewPlan = normalizedAdapterPlan(plan)
	}
	view := adapterTemplateView(viewPlan)
	tmpl, err := template.New("adapter").Parse(adapterTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, view); err != nil {
		return nil, err
	}
	adapterPath := AdapterFilePath(plan)
	rendered, err := formatGo(adapterPath, out.Bytes())
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{adapterPath: rendered}
	if plan != nil && plan.AdapterPlan != nil {
		helperPath := NormalizedHelperFilePath(plan)
		helper, err := renderNormalizedHelper(plan, helperPath)
		if err != nil {
			return nil, err
		}
		files[helperPath] = helper
	}
	return files, nil
}

func adapterTemplateView(plan *Plan) adapterView {
	allParams := make([]Param, 0, len(plan.BoundaryParams)+len(plan.ReconstructedParams))
	allParams = append(allParams, plan.BoundaryParams...)
	for _, rp := range plan.ReconstructedParams {
		allParams = append(allParams, rp.Param)
	}
	sortParamsByIndex(allParams)

	var imports []importSpec
	var paramParts []string
	var argParts []string

	if plan.ReceiverParam != nil {
		paramParts = append(paramParts, "recv "+plan.ReceiverParam.GoType)
	}

	for _, p := range allParams {
		paramParts = append(paramParts, p.Name+" "+p.GoType)
		argParts = append(argParts, p.Name)
		if p.TypePackagePath != "" && p.TypePackagePath != plan.CutPoint.PackagePath {
			imports = append(imports, importSpec{Path: p.TypePackagePath})
		}
	}

	var resultParts []string
	for _, r := range plan.Results {
		resultParts = append(resultParts, r.GoType)
		if r.TypePackagePath != "" && r.TypePackagePath != plan.CutPoint.PackagePath {
			imports = append(imports, importSpec{Path: r.TypePackagePath})
		}
	}

	resultList := ""
	switch len(resultParts) {
	case 1:
		resultList = resultParts[0]
	default:
		if len(resultParts) > 1 {
			resultList = "(" + strings.Join(resultParts, ", ") + ")"
		}
	}

	args := strings.Join(argParts, ", ")
	callTarget := renamedOriginalFunc(plan)
	if plan.AdapterPlan != nil {
		callTarget = normalizedHelperFuncName(plan)
	}
	var callExpr string
	if plan.ReceiverParam != nil {
		callExpr = "recv." + callTarget + "(" + args + ")"
	} else {
		callExpr = callTarget + "(" + args + ")"
	}

	return adapterView{
		PackageName: plan.CutPoint.PackageName,
		Imports:     uniqueImports(imports),
		AdapterName: adapterFuncName(plan.CutPoint.FuncName),
		ParamList:   strings.Join(paramParts, ", "),
		ResultList:  resultList,
		HasResults:  len(plan.Results) > 0,
		CallExpr:    callExpr,
	}
}

func NormalizedHelperFilePath(plan *Plan) string {
	return filepath.Join(plan.CutPoint.PackageDir, "monolift_normalized_"+plan.EnvServiceName+".go")
}

func normalizedHelperFuncName(plan *Plan) string {
	return "monoliftNormalized" + plan.CutPoint.FuncName
}

func renderNormalizedHelper(plan *Plan, path string) ([]byte, error) {
	body, err := normalizedHelperBody(plan)
	if err != nil {
		return nil, err
	}
	view := struct {
		PackageName string
		Imports     []importSpec
		FuncName    string
		ParamList   string
		ResultList  string
		Body        string
	}{
		PackageName: plan.CutPoint.PackageName,
		Imports: []importSpec{
			{Path: "bytes"},
			{Path: "github.com/disintegration/imaging"},
		},
		FuncName:   normalizedHelperFuncName(plan),
		ParamList:  adapterParamList(normalizedAdapterPlan(plan).BoundaryParams),
		ResultList: computeStubReturnSig(normalizedAdapterPlan(plan).Results),
		Body:       body,
	}
	tmpl, err := template.New("normalized").Parse(normalizedHelperTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, view); err != nil {
		return nil, err
	}
	return formatGo(path, out.Bytes())
}

// normalizedHelperBody parses the cut function, applies each adapter
// pattern's body rewrite (pattern-owned AST surgery — adapter.go names no
// pattern), and re-prints the rewritten body. The rewrites are dispatched
// from the AdapterPlan's input/output transforms; a pattern that needs no
// body surgery simply does not implement the rewriter interface. A pattern
// that reports it could not match its expected shape is a genuine codegen
// mismatch and aborts helper rendering rather than emitting partial output.
func normalizedHelperBody(plan *Plan) (string, error) {
	cutFile := absoluteCutFile(plan)
	src, err := os.ReadFile(cutFile)
	if err != nil {
		return "", fmt.Errorf("read cut file for normalized helper: %w", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, cutFile, src, parser.SkipObjectResolution)
	if err != nil {
		return "", fmt.Errorf("parse cut file for normalized helper: %w", err)
	}
	want := renamedOriginalFunc(plan)
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		candidate, ok := decl.(*ast.FuncDecl)
		if !ok || candidate.Name == nil {
			continue
		}
		if candidate.Name.Name == want || candidate.Name.Name == plan.CutPoint.FuncName {
			fn = candidate
			break
		}
	}
	if fn == nil || fn.Body == nil {
		return "", fmt.Errorf("codegen: function %s not found for normalized helper", plan.CutPoint.FuncName)
	}
	if plan.AdapterPlan != nil {
		if err := rewriteHelperBodyAST(fn.Body, plan); err != nil {
			return "", err
		}
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, fn.Body); err != nil {
		return "", fmt.Errorf("print normalized helper body: %w", err)
	}
	body := buf.String()
	body = strings.TrimPrefix(strings.TrimSuffix(body, "}"), "{")
	body = strings.TrimSpace(body)
	return body, nil
}

// rewriteHelperBodyAST applies each input/output transform's pattern-owned
// rewrite to the parsed helper body, mutating it in place.
func rewriteHelperBodyAST(body *ast.BlockStmt, plan *Plan) error {
	for _, transform := range plan.AdapterPlan.InputTransforms {
		pattern := adapterPatternByName(transform.Name)
		rewriter, ok := pattern.(inputBodyRewriter)
		if !ok {
			continue
		}
		normName := adapterInputName(transform, Param{Name: transform.ParamName})
		if !rewriter.rewriteInputBody(body, transform.ParamName, normName) {
			return fmt.Errorf("codegen: %s input body rewrite did not match expected shape for parameter %q", transform.Name, transform.ParamName)
		}
	}
	for _, transform := range plan.AdapterPlan.OutputTransforms {
		pattern := adapterPatternByName(transform.Name)
		rewriter, ok := pattern.(outputBodyRewriter)
		if !ok {
			continue
		}
		if !rewriter.rewriteOutputBody(body) {
			return fmt.Errorf("codegen: %s output body rewrite did not match expected shape for return type %s", transform.Name, transform.FromType)
		}
	}
	return nil
}

func adapterParamList(params []Param) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, param.Name+" "+param.GoType)
	}
	return strings.Join(parts, ", ")
}

const normalizedHelperTemplate = `package {{ .PackageName }}

import (
{{- range .Imports }}
	{{ if .Alias }}{{ .Alias }} {{ end }}"{{ .Path }}"
{{- end }}
)

func {{ .FuncName }}({{ .ParamList }}) {{ .ResultList }} {
{{ .Body }}
}
`

const adapterTemplate = `package {{ .PackageName }}
{{- if .Imports }}

import (
{{- range .Imports }}
	{{ if .Alias }}{{ .Alias }} {{ end }}"{{ .Path }}"
{{- end }}
)
{{- end }}

func {{ .AdapterName }}({{ .ParamList }}){{ if .ResultList }} {{ .ResultList }}{{ end }} {
	{{ if .HasResults }}return {{ end }}{{ .CallExpr }}
}
`
