package codegen

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"
)

func RenderClient(plan *Plan) (map[string][]byte, error) {
	view := clientTemplateView(plan)
	tmpl, err := template.New("client").Parse(clientTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, view); err != nil {
		return nil, err
	}
	rendered, err := formatGo(plan.ClientPath, out.Bytes())
	if err != nil {
		return nil, err
	}
	return map[string][]byte{plan.ClientPath: rendered}, nil
}

type clientView struct {
	Plan              *Plan
	Imports           []importSpec
	RequestFields     []fieldView
	ResponseField     fieldView
	Params            []fieldView
	ParamList         string
	BoundaryArgs      string
	OriginalArgs      string
	HasResult         bool
	LocalizedResult   bool
	PrimitiveResult   bool
	ResultType        string
	ResultZero        string
	StubName          string
	EndpointEnv       string
	EnabledEnv        string
	DefaultEndpoint   string
	NeedsErrorsImport bool
}

func clientTemplateView(plan *Plan) clientView {
	imports := []importSpec{
		{Path: "bytes"},
		{Path: "encoding/json"},
		{Path: "errors"},
		{Path: "net/http"},
		{Path: "os"},
		{Path: "time"},
	}
	var requestFields []fieldView
	for _, param := range plan.BoundaryParams {
		requestFields = append(requestFields, fieldView{
			Name:         exportedFieldName(param.Name),
			OriginalName: param.Name,
			JSONName:     param.JSONName,
			Type:         param.GoType,
			ZeroValue:    zeroValue(param.GoType),
		})
		if param.TypePackagePath != "" && param.TypePackagePath != plan.CutPoint.PackagePath {
			imports = append(imports, importSpec{Path: param.TypePackagePath})
		}
	}
	var params []fieldView
	allParams := append([]Param(nil), plan.BoundaryParams...)
	for _, param := range plan.ReconstructedParams {
		allParams = append(allParams, param.Param)
	}
	sortParamsByIndex(allParams)
	for _, param := range allParams {
		params = append(params, fieldView{Name: param.Name, Type: param.GoType, ZeroValue: zeroValue(param.GoType)})
		if param.TypePackagePath != "" && param.TypePackagePath != plan.CutPoint.PackagePath {
			imports = append(imports, importSpec{Path: param.TypePackagePath})
		}
	}
	response := fieldView{}
	hasResult := len(plan.Results) > 0
	localized := false
	resultType := ""
	resultZero := ""
	needsErrors := false
	if hasResult {
		result := plan.Results[0]
		response = fieldView{
			Name:      exportedFieldName(result.Name),
			JSONName:  result.JSONName,
			Type:      result.GoType,
			ZeroValue: zeroValue(result.GoType),
		}
		resultType = result.GoType
		resultZero = zeroValue(result.GoType)
		if result.TypePackagePath != "" && result.TypePackagePath != plan.CutPoint.PackagePath {
			imports = append(imports, importSpec{Path: result.TypePackagePath})
		}
		localized = result.Codec == CodecLocalizedErrorWrapper
		needsErrors = localized
	}
	return clientView{
		Plan:              plan,
		Imports:           uniqueImports(imports),
		RequestFields:     requestFields,
		ResponseField:     response,
		Params:            params,
		ParamList:         clientParamList(params),
		BoundaryArgs:      clientRequestLiteral(plan.BoundaryParams),
		OriginalArgs:      clientOriginalArgs(params),
		HasResult:         hasResult,
		LocalizedResult:   localized,
		PrimitiveResult:   hasResult && !localized,
		ResultType:        resultType,
		ResultZero:        resultZero,
		StubName:          stubFuncName(plan),
		EndpointEnv:       "MONOLIFT_" + plan.EnvServiceName + "_ENDPOINT",
		EnabledEnv:        "MONOLIFT_LIFT_" + plan.EnvServiceName,
		DefaultEndpoint:   "http://127.0.0.1:8081/invoke",
		NeedsErrorsImport: needsErrors,
	}
}

func sortParamsByIndex(params []Param) {
	for i := 0; i < len(params)-1; i++ {
		for j := i + 1; j < len(params); j++ {
			if params[j].Index < params[i].Index {
				params[i], params[j] = params[j], params[i]
			}
		}
	}
}

func clientParamList(params []fieldView) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, param.Name+" "+param.Type)
	}
	return strings.Join(parts, ", ")
}

func clientOriginalArgs(params []fieldView) string {
	args := make([]string, 0, len(params))
	for _, param := range params {
		args = append(args, param.Name)
	}
	return strings.Join(args, ", ")
}

func clientRequestLiteral(params []Param) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, exportedFieldName(param.Name)+": "+param.Name)
	}
	return strings.Join(parts, ", ")
}

func ClientPackageDir(plan *Plan) string {
	return filepath.Dir(plan.ClientPath)
}

const clientTemplate = `package {{ .Plan.CutPoint.PackageName }}

import (
{{- range .Imports }}
	{{ if .Alias }}{{ .Alias }} {{ end }}"{{ .Path }}"
{{- end }}
)

type monoliftInvokeRequest struct {
{{- range .RequestFields }}
	{{ .Name }} {{ .Type }} ` + "`json:\"{{ .JSONName }}\"`" + `
{{- end }}
}

type monoliftInvokeResponse struct {
{{- if .LocalizedResult }}
	Error *monoliftLocalizedError ` + "`json:\"error,omitempty\"`" + `
{{- else if .HasResult }}
	{{ .ResponseField.Name }} {{ .ResponseField.Type }} ` + "`json:\"{{ .ResponseField.JSONName }}\"`" + `
{{- end }}
}

type monoliftLocalizedError struct {
	Error   string ` + "`json:\"error,omitempty\"`" + `
	Message string ` + "`json:\"message,omitempty\"`" + `
}

func {{ .StubName }}({{ .ParamList }}) {{ .ResultType }} {
	if os.Getenv("{{ .EnabledEnv }}") != "on" {
		return {{ .Plan.CutPoint.FuncName }}({{ .OriginalArgs }})
	}
	result, err := monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .OriginalArgs }})
	if err != nil {
		if os.Getenv("MONOLIFT_LIFT_FAILMODE") == "closed" {
			return {{ .ResultZero }}
		}
		return {{ .Plan.CutPoint.FuncName }}({{ .OriginalArgs }})
	}
	return result
}

func monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .ParamList }}) ({{ .ResultType }}, error) {
	endpoint := os.Getenv("{{ .EndpointEnv }}")
	if endpoint == "" {
		endpoint = "{{ .DefaultEndpoint }}"
	}
	payload := monoliftInvokeRequest{ {{ .BoundaryArgs }} }
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return {{ .ResultZero }}, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		return {{ .ResultZero }}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return {{ .ResultZero }}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return {{ .ResultZero }}, errors.New(resp.Status)
	}
	var decoded monoliftInvokeResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return {{ .ResultZero }}, err
	}
{{- if .LocalizedResult }}
	if decoded.Error != nil {
		message := decoded.Error.Message
		if message == "" {
			message = decoded.Error.Error
		}
		return locale.NewLocalizedErrorWrapper(errors.New(decoded.Error.Error), message), nil
	}
	return nil, nil
{{- else if .HasResult }}
	return decoded.{{ .ResponseField.Name }}, nil
{{- else }}
	return nil
{{- end }}
}
`
