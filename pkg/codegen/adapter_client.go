package codegen

import (
	"bytes"
	"strings"
	"text/template"
)

func RenderAdapterClient(plan *Plan) (map[string][]byte, error) {
	view := adapterClientTemplateView(plan)
	tmpl, err := template.New("adapter-client").Parse(adapterClientTemplate)
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

type adapterClientView struct {
	Plan              *Plan
	TransportPlan     *Plan
	Imports           []importSpec
	RequestFields     []fieldView
	DTOFields         []fieldView
	OriginalParamList string
	OriginalArgs      string
	RemoteParamList   string
	RemoteArgs        string
	RemoteRequest     string
	StubReturnSig     string
	RemoteReturnSig   string
	TransportErrZeros string
	FailClosedReturn  string
	EndpointEnv       string
	EnabledEnv        string
	DefaultEndpoint   string
	ExtractionLines   []string
	RemoteCallVars    string
	ReturnExprs       string
	DecodeExprs       string
}

func adapterClientTemplateView(plan *Plan) adapterClientView {
	transport := normalizedAdapterPlan(plan)
	imports := []importSpec{
		{Path: "bytes"},
		{Path: "encoding/json"},
		{Path: "errors"},
		{Path: "fmt"},
		{Path: "io"},
		{Path: "net/http"},
		{Path: "os"},
		{Path: "time"},
	}
	for _, param := range plan.BoundaryParams {
		if param.TypePackagePath != "" && param.TypePackagePath != plan.CutPoint.PackagePath {
			imports = append(imports, importSpec{Path: param.TypePackagePath})
		}
	}
	var requestFields []fieldView
	for _, param := range transport.BoundaryParams {
		requestFields = append(requestFields, fieldView{
			Name:         exportedFieldName(param.Name),
			OriginalName: param.Name,
			JSONName:     param.JSONName,
			Type:         param.GoType,
			ZeroValue:    zeroValue(param.GoType),
		})
	}
	var dtoFields []fieldView
	for _, f := range transport.ResultDTO.Fields {
		dtoFields = append(dtoFields, fieldView{
			Name:         f.Name,
			JSONName:     f.JSONName,
			Type:         f.GoType,
			OriginalName: f.OriginalName,
			ZeroValue:    zeroValue(f.GoType),
		})
	}
	originalParams := originalAdapterParams(plan)
	remoteParams := transport.BoundaryParams
	extractionLines := adapterExtractionLines(plan, transport)
	remoteVars, returnExprs, decodeExprs := adapterReturnExpressions(plan, transport)
	return adapterClientView{
		Plan:              plan,
		TransportPlan:     transport,
		Imports:           uniqueImports(imports),
		RequestFields:     requestFields,
		DTOFields:         dtoFields,
		OriginalParamList: clientParamList(fieldsFromParams(originalParams)),
		OriginalArgs:      clientOriginalArgs(fieldsFromParams(originalParams)),
		RemoteParamList:   clientParamList(fieldsFromParams(remoteParams)),
		RemoteArgs:        clientOriginalArgs(fieldsFromParams(remoteParams)),
		RemoteRequest:     clientRequestLiteral(remoteParams),
		StubReturnSig:     computeStubReturnSig(plan.Results),
		RemoteReturnSig:   computeRemoteReturnSig(transport.Results),
		TransportErrZeros: computeTransportErrZeros(transport.Results),
		FailClosedReturn:  computeFailClosedReturn(plan.Results),
		EndpointEnv:       "MONOLIFT_" + plan.EnvServiceName + "_ENDPOINT",
		EnabledEnv:        "MONOLIFT_LIFT_" + plan.EnvServiceName,
		DefaultEndpoint:   "http://127.0.0.1:8081/invoke",
		ExtractionLines:   extractionLines,
		RemoteCallVars:    remoteVars,
		ReturnExprs:       returnExprs,
		DecodeExprs:       decodeExprs,
	}
}

func fieldsFromParams(params []Param) []fieldView {
	fields := make([]fieldView, 0, len(params))
	for _, param := range params {
		fields = append(fields, fieldView{Name: param.Name, Type: param.GoType, ZeroValue: zeroValue(param.GoType)})
	}
	return fields
}

func originalAdapterParams(plan *Plan) []Param {
	params := make([]Param, 0, len(plan.BoundaryParams)+len(plan.ReconstructedParams))
	params = append(params, plan.BoundaryParams...)
	for _, rp := range plan.ReconstructedParams {
		params = append(params, rp.Param)
	}
	sortParamsByIndex(params)
	return params
}

func adapterExtractionLines(plan, transport *Plan) []string {
	byOriginalName := map[string]Param{}
	for _, param := range plan.BoundaryParams {
		byOriginalName[param.Name] = param
	}
	var lines []string
	for _, transform := range plan.AdapterPlan.InputTransforms {
		var out Param
		for _, param := range transport.BoundaryParams {
			if param.Name == adapterInputName(transform, byOriginalName[transform.ParamName]) {
				out = param
				break
			}
		}
		pattern := adapterPatternByName(transform.Name)
		if pattern == nil {
			continue
		}
		lines = append(lines, pattern.RenderInputExtraction(transform.ParamName, out.Name, "return "+zeroTupleWithErr(plan.Results, "%s"))...)
		lines = append(lines,
			"if len("+out.Name+") > 8*1024*1024 {",
			"\treturn "+zeroTupleWithErr(plan.Results, `fmt.Errorf("monolift: adapter payload exceeds 8 MiB limit")`),
			"}",
		)
	}
	return lines
}

func adapterPatternByName(name string) AdapterPatternImpl {
	for _, pattern := range adapterPatternRegistry {
		if pattern.Name() == name {
			return pattern
		}
	}
	return nil
}

func zeroTupleWithErr(results []Result, errExpr string) string {
	parts := make([]string, len(results))
	for i, result := range results {
		if result.Codec == CodecError {
			parts[i] = errExpr
		} else {
			parts[i] = zeroValue(result.GoType)
		}
	}
	return strings.Join(parts, ", ")
}

func adapterReturnExpressions(plan, transport *Plan) (remoteVars, returnExprs, decodeExprs string) {
	var remote []string
	var ret []string
	var decoded []string
	outputByType := map[string]AdapterPattern{}
	for _, transform := range plan.AdapterPlan.OutputTransforms {
		outputByType[transform.FromType] = transform
	}
	nonErrorIndex := 0
	for _, result := range transport.Results {
		if result.Codec == CodecError {
			continue
		}
		v := "r" + string(rune('0'+nonErrorIndex))
		remote = append(remote, v)
		decoded = append(decoded, "decoded."+transport.ResultDTO.Fields[nonErrorIndex].Name)
		original := plan.Results[result.Index]
		if transform, ok := outputByType[original.GoType]; ok {
			pattern := adapterPatternByName(transform.Name)
			if pattern != nil {
				ret = append(ret, pattern.RenderRemoteReconstruction(v))
			} else {
				ret = append(ret, v)
			}
		} else {
			ret = append(ret, v)
		}
		nonErrorIndex++
	}
	hasErr := false
	for _, result := range plan.Results {
		if result.Codec == CodecError {
			hasErr = true
			break
		}
	}
	if hasErr {
		remote = append(remote, "appErr")
		ret = append(ret, "appErr")
	}
	return strings.Join(append(remote, "transportErr"), ", "), strings.Join(ret, ", "), strings.Join(decoded, ", ")
}

const adapterClientTemplate = `package {{ .Plan.CutPoint.PackageName }}

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
{{- range .DTOFields }}
	{{ .Name }} {{ .Type }} ` + "`json:\"{{ .JSONName }}\"`" + `
{{- end }}
	Error string ` + "`json:\"error,omitempty\"`" + `
}

func {{ .Plan.CutPoint.FuncName }}({{ .OriginalParamList }}) {{ .StubReturnSig }} {
	if os.Getenv("{{ .EnabledEnv }}") != "on" {
		return {{ .Plan.CutPoint.FuncName | printf "monoliftOriginal%s" }}({{ .OriginalArgs }})
	}
{{- range .ExtractionLines }}
	{{ . }}
{{- end }}
	{{ .RemoteCallVars }} := monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .RemoteArgs }})
	if transportErr != nil {
		if os.Getenv("MONOLIFT_LIFT_FAILMODE") == "closed" {
			return {{ .FailClosedReturn }}
		}
		return {{ .Plan.CutPoint.FuncName | printf "monoliftOriginal%s" }}({{ .OriginalArgs }})
	}
	return {{ .ReturnExprs }}
}

func monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .RemoteParamList }}) {{ .RemoteReturnSig }} {
	endpoint := os.Getenv("{{ .EndpointEnv }}")
	if endpoint == "" {
		endpoint = "{{ .DefaultEndpoint }}"
	}
	payload := monoliftInvokeRequest{ {{ .RemoteRequest }} }
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return {{ .TransportErrZeros }}err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		return {{ .TransportErrZeros }}err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return {{ .TransportErrZeros }}err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return {{ .TransportErrZeros }}errors.New(resp.Status)
	}
	var decoded monoliftInvokeResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return {{ .TransportErrZeros }}err
	}
	var appErr error
	if decoded.Error != "" {
		appErr = errors.New(decoded.Error)
	}
	return {{ .DecodeExprs }}, appErr, nil
}
`
