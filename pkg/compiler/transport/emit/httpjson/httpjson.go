package httpjson

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/tgoodwin/monolift/pkg/compiler/transport"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

func init() {
	emit.Register(transport.TemplateHTTPJSON, Render)
}

func Render(ctx emit.Context) (emit.Artifact, error) {
	files := map[string][]byte{}
	for _, spec := range []struct {
		template string
		path     string
		gofmt    bool
	}{
		{"main.go.tmpl", "extracted-cleanpath/main.go", true},
		{"gomod.tmpl", "extracted-cleanpath/go.mod", false},
		{"dockerfile.tmpl", "extracted-cleanpath/Dockerfile", false},
		{"service.yaml.tmpl", "manifests/extracted-service.yaml", false},
		{"deployment.yaml.tmpl", "manifests/extracted-deployment.yaml", false},
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
		"lower": strings.ToLower,
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
	}
}
