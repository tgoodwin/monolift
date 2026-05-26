package codegen

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"
)

func RenderClient(plan *Plan) (map[string][]byte, error) {
	if plan != nil && plan.AdapterPlan != nil {
		return RenderAdapterClient(plan)
	}
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
	Plan                 *Plan
	Imports              []importSpec
	RequestFields        []fieldView
	ResponseField        fieldView
	Params               []fieldView
	ParamList            string
	BoundaryArgs         string
	OriginalArgs         string
	HasResult            bool
	HasErrorResult       bool
	HasVoidReturn        bool
	LocalizedResult      bool
	PrimitiveResult      bool
	ResultType           string
	ResultZero           string
	StubReturnSig        string
	RemoteReturnSig      string
	TransportErrZeros    string
	FailClosedReturn     string
	StubName             string
	OriginalFuncName     string
	EndpointEnv          string
	EnabledEnv           string
	DefaultEndpoint      string
	NeedsErrorsImport    bool
	StreamingBytesParams []streamingByteParam

	// ResultDTO support
	HasDTO         bool
	DTOFields      []fieldView
	DTORemoteVars  string // LHS of monoliftRemote call: "r0, r1, appErr, transportErr"
	DTOStubVars    string // LHS of monoliftRemote call in stub: "r0, r1, appErr, transportErr"
	DTOReturnVars  string // "r0, r1, appErr" — what the stub returns on success
	DTODecodeExprs string // "decoded.Field0, decoded.Field1"

	// Receiver support
	HasReceiver            bool
	ReceiverDecl           string // e.g. "(t TemplateContext) " or "(h *Argon2Hasher) "
	ReceiverSelf           string // e.g. "t" or "h" — the receiver variable name
	ReceiverRequestType    string // qualified type for request field (ReceiverBoundary only)
	ReceiverBoundaryAssign string // "Receiver: t, " for boundary, empty otherwise
	CallPrefix             string // "t." for methods, "" for free functions
}

type streamingByteParam struct {
	Name    string
	ByteVar string
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
	var streamingParams []streamingByteParam
	for _, param := range plan.BoundaryParams {
		fieldType := param.GoType
		if param.Codec == CodecStreamingBytes {
			fieldType = "[]byte"
			streamingParams = append(streamingParams, streamingByteParam{
				Name:    param.Name,
				ByteVar: param.Name + "Bytes",
			})
		}
		requestFields = append(requestFields, fieldView{
			Name:         exportedFieldName(param.Name),
			OriginalName: param.Name,
			JSONName:     param.JSONName,
			Type:         fieldType,
			ZeroValue:    zeroValue(param.GoType),
		})
		if param.Codec != CodecStreamingBytes && param.TypePackagePath != "" && param.TypePackagePath != plan.CutPoint.PackagePath {
			imports = append(imports, importSpec{Path: param.TypePackagePath})
		}
	}
	if len(streamingParams) > 0 {
		imports = append(imports, importSpec{Path: "fmt"})
		imports = append(imports, importSpec{Path: "io"})
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
	// Separate non-error results from error results.
	response := fieldView{}
	hasNonErrorResult := false
	hasErrorResult := false
	localized := false
	resultType := ""
	resultZero := ""
	needsErrors := false
	hasDTO := plan.ResultDTO != nil
	for _, result := range plan.Results {
		if result.Codec == CodecError {
			hasErrorResult = true
			continue
		}
		if hasDTO {
			// ResultDTO handles multi-value; add imports for all result types.
			if result.TypePackagePath != "" && result.TypePackagePath != plan.CutPoint.PackagePath {
				imports = append(imports, importSpec{Path: result.TypePackagePath})
			}
			hasNonErrorResult = true
			continue
		}
		if !hasNonErrorResult {
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
			hasNonErrorResult = true
		}
	}
	if hasErrorResult {
		imports = append(imports, importSpec{Path: "fmt"})
		needsErrors = true // errors.New for reconstructing app errors
	}

	// Build DTO field views and call/return metadata.
	var dtoFields []fieldView
	var dtoRemoteVars, dtoStubVars, dtoReturnVars, dtoDecodeExprs string
	if hasDTO {
		for _, f := range plan.ResultDTO.Fields {
			dtoFields = append(dtoFields, fieldView{
				Name:         f.Name,
				JSONName:     f.JSONName,
				Type:         f.GoType,
				OriginalName: f.OriginalName,
				ZeroValue:    zeroValue(f.GoType),
			})
		}
		// Build LHS vars: r0, r1, ..., appErr, transportErr
		var remoteVars, returnVars []string
		var decodeExprs []string
		for i, f := range plan.ResultDTO.Fields {
			v := "r" + string(rune('0'+i))
			remoteVars = append(remoteVars, v)
			returnVars = append(returnVars, v)
			decodeExprs = append(decodeExprs, "decoded."+f.Name)
		}
		if hasErrorResult {
			remoteVars = append(remoteVars, "appErr")
			returnVars = append(returnVars, "appErr")
		}
		dtoStubVars = strings.Join(remoteVars, ", ") + ", transportErr"
		remoteVars = append(remoteVars, "transportErr") // for remote function LHS (not needed separately)
		dtoRemoteVars = strings.Join(remoteVars, ", ")
		dtoReturnVars = strings.Join(returnVars, ", ")
		dtoDecodeExprs = strings.Join(decodeExprs, ", ")
	}

	hasVoidReturn := !hasNonErrorResult && !hasErrorResult
	view := clientView{
		Plan:                 plan,
		Imports:              uniqueImports(imports),
		RequestFields:        requestFields,
		ResponseField:        response,
		Params:               params,
		ParamList:            clientParamList(params),
		BoundaryArgs:         clientRequestLiteral(plan.BoundaryParams),
		OriginalArgs:         clientOriginalArgs(params),
		HasResult:            hasNonErrorResult,
		HasErrorResult:       hasErrorResult,
		HasVoidReturn:        hasVoidReturn,
		LocalizedResult:      localized,
		PrimitiveResult:      hasNonErrorResult && !localized,
		ResultType:           resultType,
		ResultZero:           resultZero,
		StubReturnSig:        computeStubReturnSig(plan.Results),
		RemoteReturnSig:      computeRemoteReturnSig(plan.Results),
		TransportErrZeros:    computeTransportErrZeros(plan.Results),
		FailClosedReturn:     computeFailClosedReturn(plan.Results),
		StubName:             plan.CutPoint.FuncName,
		OriginalFuncName:     renamedOriginalFunc(plan),
		EndpointEnv:          "MONOLIFT_" + plan.EnvServiceName + "_ENDPOINT",
		EnabledEnv:           "MONOLIFT_LIFT_" + plan.EnvServiceName,
		DefaultEndpoint:      "http://127.0.0.1:8081/invoke",
		NeedsErrorsImport:    needsErrors,
		StreamingBytesParams: streamingParams,
		HasDTO:               hasDTO,
		DTOFields:            dtoFields,
		DTORemoteVars:        dtoRemoteVars,
		DTOStubVars:          dtoStubVars,
		DTOReturnVars:        dtoReturnVars,
		DTODecodeExprs:       dtoDecodeExprs,
	}

	// Receiver support: generate method stubs when a receiver is present.
	if plan.ReceiverParam != nil {
		view.HasReceiver = true
		baseType := strings.TrimPrefix(plan.ReceiverParam.GoType, "*")
		receiverVar := strings.ToLower(baseType[:1])

		if plan.ReceiverParam.IsPointer {
			view.ReceiverDecl = "(" + receiverVar + " *" + baseType + ") "
		} else {
			view.ReceiverDecl = "(" + receiverVar + " " + baseType + ") "
		}
		view.ReceiverSelf = receiverVar

		view.CallPrefix = receiverVar + "."

		if plan.ReceiverParam.Policy == ReceiverBoundary {
			view.ReceiverRequestType = baseType
			view.ReceiverBoundaryAssign = "Receiver: " + receiverVar + ", "
		}
	}

	return view
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
		value := param.Name
		if param.Codec == CodecStreamingBytes {
			value = param.Name + "Bytes"
		}
		parts = append(parts, exportedFieldName(param.Name)+": "+value)
	}
	return strings.Join(parts, ", ")
}

// computeStubReturnSig returns the Go return type signature for the stub function.
// Examples: "string", "(string, error)", "", "error".
func computeStubReturnSig(results []Result) string {
	if len(results) == 0 {
		return ""
	}
	if len(results) == 1 {
		return results[0].GoType
	}
	parts := make([]string, len(results))
	for i, r := range results {
		parts[i] = r.GoType
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// computeRemoteReturnSig returns the Go return type for the monoliftRemote function.
// Appends a transport error to the function's results.
// Examples: "(string, error)" for single string, "(string, error, error)" for (string, error),
// "error" for void.
func computeRemoteReturnSig(results []Result) string {
	parts := make([]string, 0, len(results)+1)
	for _, r := range results {
		parts = append(parts, r.GoType)
	}
	parts = append(parts, "error") // transport error
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// computeTransportErrZeros returns the zero-value prefix for transport error returns
// in monoliftRemote. Examples: `"", nil, ` for (string, error); `"", ` for string;
// "" for void.
func computeTransportErrZeros(results []Result) string {
	if len(results) == 0 {
		return ""
	}
	parts := make([]string, len(results))
	for i, r := range results {
		parts[i] = zeroValue(r.GoType)
	}
	return strings.Join(parts, ", ") + ", "
}

// computeFailClosedReturn returns the expression for fail-closed return values.
// For (T, error): `"", fmt.Errorf("monolift: extracted service unavailable")`.
// For single T: same as zeroValue. For void: "".
func computeFailClosedReturn(results []Result) string {
	if len(results) == 0 {
		return ""
	}
	parts := make([]string, len(results))
	for i, r := range results {
		if r.Codec == CodecError {
			parts[i] = `fmt.Errorf("monolift: extracted service unavailable")`
		} else {
			parts[i] = zeroValue(r.GoType)
		}
	}
	return strings.Join(parts, ", ")
}

func ClientPackageDir(plan *Plan) string {
	return filepath.Dir(plan.ClientPath)
}

// clientTemplate renders the non-adapter lift client. It is intentionally
// kept separate from adapterClientTemplate (adapter_client.go): the two render
// from different view types (clientView vs adapterClientView) and handle
// different result-shape families — this one branches across void / single /
// (T,error) / DTO / localized-error returns, while the adapter client always
// emits a DTO-shaped response plus host-side input extraction and return
// reconstruction. Unifying them behind {{ if .HasAdapter }} would force a
// single merged view and interleave two unrelated branch sets, so they are
// forked deliberately. The shared transport/plumbing (endpoint env lookup,
// HTTP POST headers, client timeout, status check, fail-mode handling) MUST
// stay byte-identical between the two; TestClientTemplatesShareTransportPlumbing
// is the golden cross-check guarding that invariant against drift.
const clientTemplate = `package {{ .Plan.CutPoint.PackageName }}

import (
{{- range .Imports }}
	{{ if .Alias }}{{ .Alias }} {{ end }}"{{ .Path }}"
{{- end }}
)

type monoliftInvokeRequest struct {
{{- if .ReceiverRequestType }}
	Receiver {{ .ReceiverRequestType }} ` + "`json:\"receiver\"`" + `
{{- end }}
{{- range .RequestFields }}
	{{ .Name }} {{ .Type }} ` + "`json:\"{{ .JSONName }}\"`" + `
{{- end }}
}

type monoliftInvokeResponse struct {
{{- if .LocalizedResult }}
	Error *monoliftLocalizedError ` + "`json:\"error,omitempty\"`" + `
{{- else if .HasDTO }}
{{- range .DTOFields }}
	{{ .Name }} {{ .Type }} ` + "`json:\"{{ .JSONName }}\"`" + `
{{- end }}
{{- if .HasErrorResult }}
	Error string ` + "`json:\"error,omitempty\"`" + `
{{- end }}
{{- else }}
{{- if .HasResult }}
	{{ .ResponseField.Name }} {{ .ResponseField.Type }} ` + "`json:\"{{ .ResponseField.JSONName }}\"`" + `
{{- end }}
{{- if .HasErrorResult }}
	Error string ` + "`json:\"error,omitempty\"`" + `
{{- end }}
{{- end }}
}

type monoliftLocalizedError struct {
	Error   string ` + "`json:\"error,omitempty\"`" + `
	Message string ` + "`json:\"message,omitempty\"`" + `
}

func {{ .ReceiverDecl }}{{ .StubName }}({{ .ParamList }}){{ if .StubReturnSig }} {{ .StubReturnSig }}{{ end }} {
{{- if .HasVoidReturn }}
	if os.Getenv("{{ .EnabledEnv }}") != "on" {
		{{ .CallPrefix }}{{ .OriginalFuncName }}({{ .OriginalArgs }})
		return
	}
	if err := {{ .CallPrefix }}monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .OriginalArgs }}); err != nil {
		if os.Getenv("MONOLIFT_LIFT_FAILMODE") != "closed" {
			{{ .CallPrefix }}{{ .OriginalFuncName }}({{ .OriginalArgs }})
		}
	}
{{- else if .HasDTO }}
	if os.Getenv("{{ .EnabledEnv }}") != "on" {
		return {{ .CallPrefix }}{{ .OriginalFuncName }}({{ .OriginalArgs }})
	}
	{{ .DTOStubVars }} := {{ .CallPrefix }}monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .OriginalArgs }})
	if transportErr != nil {
		if os.Getenv("MONOLIFT_LIFT_FAILMODE") == "closed" {
			return {{ .FailClosedReturn }}
		}
		return {{ .CallPrefix }}{{ .OriginalFuncName }}({{ .OriginalArgs }})
	}
	return {{ .DTOReturnVars }}
{{- else if .HasErrorResult }}
	if os.Getenv("{{ .EnabledEnv }}") != "on" {
		return {{ .CallPrefix }}{{ .OriginalFuncName }}({{ .OriginalArgs }})
	}
{{- if .HasResult }}
	result, appErr, transportErr := {{ .CallPrefix }}monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .OriginalArgs }})
{{- else }}
	appErr, transportErr := {{ .CallPrefix }}monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .OriginalArgs }})
{{- end }}
	if transportErr != nil {
		if os.Getenv("MONOLIFT_LIFT_FAILMODE") == "closed" {
			return {{ .FailClosedReturn }}
		}
		return {{ .CallPrefix }}{{ .OriginalFuncName }}({{ .OriginalArgs }})
	}
{{- if .HasResult }}
	return result, appErr
{{- else }}
	return appErr
{{- end }}
{{- else }}
	if os.Getenv("{{ .EnabledEnv }}") != "on" {
		return {{ .CallPrefix }}{{ .OriginalFuncName }}({{ .OriginalArgs }})
	}
	result, err := {{ .CallPrefix }}monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .OriginalArgs }})
	if err != nil {
		if os.Getenv("MONOLIFT_LIFT_FAILMODE") == "closed" {
			return {{ .ResultZero }}
		}
		return {{ .CallPrefix }}{{ .OriginalFuncName }}({{ .OriginalArgs }})
	}
	return result
{{- end }}
}

func {{ .ReceiverDecl }}monoliftRemote{{ .Plan.CutPoint.FuncName }}({{ .ParamList }}) {{ .RemoteReturnSig }} {
	endpoint := os.Getenv("{{ .EndpointEnv }}")
	if endpoint == "" {
		endpoint = "{{ .DefaultEndpoint }}"
	}
{{- range .StreamingBytesParams }}
	{{ .ByteVar }}, err := io.ReadAll({{ .Name }})
	if err != nil {
		return {{ $.TransportErrZeros }}fmt.Errorf("monolift: read streaming param {{ .Name }}: %w", err)
	}
	if len({{ .ByteVar }}) > 10*1024*1024 {
		return {{ $.TransportErrZeros }}fmt.Errorf("monolift: streaming param {{ .Name }} exceeds 10MB limit")
	}
{{- end }}
	payload := monoliftInvokeRequest{ {{ .ReceiverBoundaryAssign }}{{ .BoundaryArgs }} }
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
{{- if .LocalizedResult }}
	if decoded.Error != nil {
		message := decoded.Error.Message
		if message == "" {
			message = decoded.Error.Error
		}
		return locale.NewLocalizedErrorWrapper(errors.New(decoded.Error.Error), message), nil
	}
	return nil, nil
{{- else if .HasDTO }}
{{- if .HasErrorResult }}
	var appErr error
	if decoded.Error != "" {
		appErr = errors.New(decoded.Error)
	}
	return {{ .DTODecodeExprs }}, appErr, nil
{{- else }}
	return {{ .DTODecodeExprs }}, nil
{{- end }}
{{- else if .HasErrorResult }}
	var appErr error
	if decoded.Error != "" {
		appErr = errors.New(decoded.Error)
	}
{{- if .HasResult }}
	return decoded.{{ .ResponseField.Name }}, appErr, nil
{{- else }}
	return appErr, nil
{{- end }}
{{- else if .HasResult }}
	return decoded.{{ .ResponseField.Name }}, nil
{{- else }}
	return nil
{{- end }}
}
`
