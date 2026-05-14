package codegen

import (
	"bytes"
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
	view := adapterTemplateView(plan)
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
	return map[string][]byte{adapterPath: rendered}, nil
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
