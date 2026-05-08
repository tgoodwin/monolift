package codegen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"text/template"
)

func RenderDockerfiles(plan *Plan) (map[string][]byte, error) {
	if plan == nil {
		return nil, fmt.Errorf("codegen: nil plan")
	}
	view := dockerfileView{Plan: plan, GoVersion: goModVersion(plan.SourceModuleRoot)}
	files := map[string][]byte{}
	if err := renderDockerfile(files, plan.ExtractedDockerfilePath, extractedDockerfileTemplate, view); err != nil {
		return nil, err
	}
	if err := renderDockerfile(files, plan.HostDockerfilePath, hostDockerfileTemplate, view); err != nil {
		return nil, err
	}
	return files, nil
}

type dockerfileView struct {
	Plan      *Plan
	GoVersion string
}

func renderDockerfile(files map[string][]byte, path, source string, view dockerfileView) error {
	tmpl, err := template.New("dockerfile").Funcs(template.FuncMap{
		"dockerQuote": strconv.Quote,
	}).Parse(source)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, view); err != nil {
		return err
	}
	files[path] = out.Bytes()
	return nil
}

var goDirectivePattern = regexp.MustCompile(`(?m)^go[ \t]+([0-9]+(?:\.[0-9]+){1,2})[ \t]*$`)

func goModVersion(moduleRoot string) string {
	if moduleRoot == "" {
		return "1.24"
	}
	data, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return "1.24"
	}
	match := goDirectivePattern.FindSubmatch(data)
	if len(match) != 2 {
		return "1.24"
	}
	return string(match[1])
}

const extractedDockerfileTemplate = `FROM golang:{{ .GoVersion }} AS builder

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -o /out/{{ .Plan.ServiceName }} ./cmd/{{ .Plan.ServiceName }}

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/{{ .Plan.ServiceName }} /{{ .Plan.ServiceName }}
EXPOSE {{ .Plan.Deploy.ExtractedPort }}
ENTRYPOINT ["/{{ .Plan.ServiceName }}"]
`

const hostDockerfileTemplate = `FROM golang:{{ .GoVersion }} AS builder

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -o /out/{{ .Plan.Deploy.HostBinaryName }} {{ .Plan.Deploy.HostBuildPackage }}
{{- range .Plan.Deploy.HostAssetCopies }}
RUN chmod -R a+rX /src/{{ .From }}
{{- end }}

FROM {{ .Plan.Deploy.HostRuntimeImage }}
{{- range .Plan.Deploy.HostRuntimeSetup }}
RUN {{ . }}
{{- end }}
COPY --from=builder /out/{{ .Plan.Deploy.HostBinaryName }} /{{ .Plan.Deploy.HostBinaryName }}
{{- range .Plan.Deploy.HostAssetCopies }}
COPY --from=builder /src/{{ .From }} {{ .To }}
{{- end }}
{{- range .Plan.Deploy.HostEnvVars }}
ENV {{ .Name }}={{ dockerQuote .Value }}
{{- end }}
EXPOSE {{ .Plan.Deploy.HostPort }}
ENTRYPOINT ["/{{ .Plan.Deploy.HostBinaryName }}"]
`
