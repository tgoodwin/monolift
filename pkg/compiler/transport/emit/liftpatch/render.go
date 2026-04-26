package liftpatch

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

func Render(ctx emit.Context) (emit.Artifact, error) {
	name := generatedSiblingName(ctx.ObjectName)
	source, err := RenderTemplate(ctx, "monolift_lift.go.tmpl")
	if err != nil {
		return emit.Artifact{}, err
	}
	source, err = format.Source(source)
	if err != nil {
		return emit.Artifact{}, fmt.Errorf("format lift client: %w", err)
	}
	files := map[string][]byte{name: source}
	paths := []string{name}
	sort.Strings(paths)
	op := emit.HostPatchOp{
		PackageImportPath: ctx.SymbolImportPath,
		FuncName:          ctx.ObjectName,
		ExpectedSignature: expectedSignature(ctx),
		PreludeSource:     preludeSource(ctx),
		GeneratedFiles:    paths,
		SentinelIdent:     "monoliftLiftEnabled",
	}
	return emit.Artifact{
		Files:        files,
		HostPatchOps: []emit.HostPatchOp{op},
		Manifest: emit.Manifest{
			ServiceName: ctx.ServiceName,
			Files:       paths,
		},
	}, nil
}

func RenderTemplate(ctx emit.Context, name string) ([]byte, error) {
	raw, err := templateFS.ReadFile(filepath.Join("templates", name))
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"camel":       lowerCamel,
		"dialer":      dialerName,
		"endpointEnv": endpointEnv,
		"lower":       strings.ToLower,
	}).Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, view(ctx)); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type templateView struct {
	emit.Context
	ResultField emit.FieldSpec
}

func view(ctx emit.Context) templateView {
	result := emit.FieldSpec{Name: "Result", JSONName: "result", GoType: "string"}
	if len(ctx.ResultFields) > 0 {
		result = ctx.ResultFields[0]
	}
	return templateView{Context: ctx, ResultField: result}
}

func generatedSiblingName(objectName string) string {
	return "monolift_lift_" + strings.ToLower(objectName) + ".go"
}

func dialerName(objectName string) string {
	return "monoliftLift" + objectName
}

func endpointEnv(prefix string) string {
	return prefix + "_ENDPOINT"
}

func lowerCamel(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func expectedSignature(ctx emit.Context) string {
	params := make([]string, 0, len(ctx.ParamFields))
	for _, field := range ctx.ParamFields {
		params = append(params, field.GoType)
	}
	results := make([]string, 0, len(ctx.ResultFields))
	for _, field := range ctx.ResultFields {
		results = append(results, field.GoType)
	}
	switch len(results) {
	case 0:
		return fmt.Sprintf("func(%s)", strings.Join(params, ", "))
	case 1:
		return fmt.Sprintf("func(%s) %s", strings.Join(params, ", "), results[0])
	default:
		return fmt.Sprintf("func(%s) (%s)", strings.Join(params, ", "), strings.Join(results, ", "))
	}
}

func preludeSource(ctx emit.Context) string {
	args := make([]string, 0, len(ctx.ParamFields))
	for _, field := range ctx.ParamFields {
		args = append(args, lowerCamel(field.Name))
	}
	return fmt.Sprintf(`if monoliftLiftEnabled {
	if result, ok := %s(%s); ok {
		return result
	}
	if !monoliftLiftFailOpen {
		return monoliftLiftFailureSentinel
	}
}`, dialerName(ctx.ObjectName), strings.Join(args, ", "))
}
