package httpjson

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"github.com/tgoodwin/monolift/pkg/compiler/transport"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

func init() {
	emit.Register(transport.TemplateHTTPJSON, Render)
}

func Render(ctx emit.Context) (emit.Artifact, error) {
	suffix := serviceSuffix(ctx)
	files := map[string][]byte{}
	for _, spec := range []struct {
		template string
		path     string
		gofmt    bool
	}{
		{"main.go.tmpl", path.Join("cmd", ctx.ServiceName, "main.go"), true},
		{"dockerfile.tmpl", "Dockerfile.extracted-" + suffix, false},
		{"service.yaml.tmpl", path.Join("manifests", "extracted-"+suffix+"-service.yaml"), false},
		{"deployment.yaml.tmpl", path.Join("manifests", "extracted-"+suffix+"-deployment.yaml"), false},
	} {
		rendered, err := RenderTemplate(ctx, spec.template)
		if err != nil {
			return emit.Artifact{}, err
		}
		if err := ValidateNoSyntheticBody(rendered); err != nil {
			return emit.Artifact{}, err
		}
		if spec.gofmt {
			rendered, err = format.Source(rendered)
			if err != nil {
				return emit.Artifact{}, fmt.Errorf("format %s: %w", spec.template, err)
			}
		}
		files[spec.path] = rendered
	}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return emit.Artifact{
		Files: files,
		Manifest: emit.Manifest{
			ServiceName: ctx.ServiceName,
			Files:       paths,
		},
	}, nil
}

func RenderTemplate(ctx emit.Context, name string) ([]byte, error) {
	raw, err := templateFS.ReadFile(filepath.Join("templates", name))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", emit.ErrTemplateUnsupported, name)
	}
	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"lower":      strings.ToLower,
		"lowerCamel": lowerCamel,
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

func ValidateNoSyntheticBody(src []byte) error {
	text := string(src)
	for _, needle := range []string{
		"func CleanPath(",
		"return cleanPath(",
		"path.Clean(",
	} {
		if strings.Contains(text, needle) {
			return fmt.Errorf("httpjson render rejected synthetic CleanPath body")
		}
	}
	return nil
}

type templateView struct {
	emit.Context
	RequestFields []emit.FieldSpec
	ResultField   emit.FieldSpec
	PackageAlias  string
	ServiceSuffix string
	CommandTarget string
}

func view(ctx emit.Context) templateView {
	result := emit.FieldSpec{Name: "Result", JSONName: "result", GoType: "string"}
	if len(ctx.ResultFields) > 0 {
		result = ctx.ResultFields[0]
	}
	return templateView{
		Context:       ctx,
		RequestFields: ctx.ParamFields,
		ResultField:   result,
		PackageAlias:  packageAlias(ctx.SymbolImportPath),
		ServiceSuffix: serviceSuffix(ctx),
		CommandTarget: ctx.ServiceName,
	}
}

func packageAlias(importPath string) string {
	return path.Base(importPath)
}

func serviceSuffix(ctx emit.Context) string {
	suffix := strings.TrimPrefix(ctx.ServiceName, "monolift-extracted-")
	if suffix == "" {
		return strings.ToLower(ctx.ObjectName)
	}
	return suffix
}

func lowerCamel(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
