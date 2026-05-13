package codegen

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"
)

func RenderServer(plan *Plan) (map[string][]byte, error) {
	view := serverTemplateView(plan)
	tmpl, err := template.New("server").Parse(serverTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, view); err != nil {
		return nil, err
	}
	rendered, err := formatGo(plan.ServerPath, out.Bytes())
	if err != nil {
		return nil, err
	}
	return map[string][]byte{plan.ServerPath: rendered}, nil
}

type serverView struct {
	Plan              *Plan
	Imports           []importSpec
	RequestFields     []fieldView
	ResponseField     fieldView
	StateFields       []fieldView
	StateInitLines    []string
	StateCloseLines   []string
	CallArgs          string
	HasResult         bool
	LocalizedResult   bool
	PrimitiveResult   bool
	CutPackageAlias   string
	GeneratedFunction string
}

type fieldView struct {
	Name          string
	JSONName      string
	Type          string
	OriginalName  string
	ZeroValue     string
	Reconstructor Reconstructor
}

func serverTemplateView(plan *Plan) serverView {
	imports := []importSpec{
		{Path: "encoding/json"},
		{Path: "log"},
		{Path: "net/http"},
		{Path: "os"},
		{Path: "sync"},
		{Path: plan.CutPoint.PackagePath},
	}
	hasStreamingBytes := false
	var requestFields []fieldView
	for _, param := range plan.BoundaryParams {
		fieldType := param.QualifiedGoType
		if param.Codec == CodecStreamingBytes {
			fieldType = "[]byte"
			hasStreamingBytes = true
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
	if hasStreamingBytes {
		imports = append(imports, importSpec{Path: "bytes"})
	}
	var stateFields []fieldView
	var stateInitLines []string
	var stateCloseLines []string
	for _, param := range plan.ReconstructedParams {
		stateFields = append(stateFields, fieldView{
			Name:          exportedFieldName(param.Name),
			OriginalName:  param.Name,
			Type:          param.QualifiedGoType,
			Reconstructor: param.Reconstructor,
		})
		if param.TypePackagePath != "" {
			imports = append(imports, importSpec{Path: param.TypePackagePath})
		}
		for _, raw := range param.Reconstructor.Imports {
			imports = append(imports, importSpecFromRaw(raw))
		}
		stateInitLines = append(stateInitLines, serverReconstructorInit(param)...)
		if param.Reconstructor.CloseSource != "" {
			stateCloseLines = append(stateCloseLines, strings.ReplaceAll(param.Reconstructor.CloseSource, "db", strings.ToLower(param.Name)+"DB"))
		}
	}
	response := fieldView{}
	hasResult := len(plan.Results) > 0
	localized := false
	primitive := false
	if hasResult {
		result := plan.Results[0]
		response = fieldView{
			Name:      exportedFieldName(result.Name),
			JSONName:  result.JSONName,
			Type:      result.QualifiedGoType,
			ZeroValue: zeroValue(result.GoType),
		}
		if result.Codec != CodecLocalizedErrorWrapper && result.TypePackagePath != "" && result.TypePackagePath != plan.CutPoint.PackagePath {
			imports = append(imports, importSpec{Path: result.TypePackagePath})
		}
		localized = result.Codec == CodecLocalizedErrorWrapper
		primitive = result.Codec != CodecLocalizedErrorWrapper
	}
	return serverView{
		Plan:              plan,
		Imports:           uniqueImports(imports),
		RequestFields:     requestFields,
		ResponseField:     response,
		StateFields:       stateFields,
		StateInitLines:    stateInitLines,
		StateCloseLines:   stateCloseLines,
		CallArgs:          strings.Join(callArgs(plan.BoundaryParams, plan.ReconstructedParams, "state."), ", "),
		HasResult:         hasResult,
		LocalizedResult:   localized,
		PrimitiveResult:   primitive,
		CutPackageAlias:   plan.CutPoint.PackageName,
		GeneratedFunction: plan.CutPoint.FuncName,
	}
}

func serverReconstructorInit(param ReconstructedParam) []string {
	field := exportedFieldName(param.Name)
	dbVar := strings.ToLower(param.Name) + "DB"
	switch param.Reconstructor.ID {
	case "sql_db":
		return []string{
			dbVar + `, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))`,
			"if err != nil { return nil, err }",
			"state." + field + " = " + dbVar,
		}
	case "sql_db_wrapper":
		alias := param.Reconstructor.ConstructorPackageAlias
		if alias == "" {
			alias = packageAlias(param.TypePackagePath)
		}
		return []string{
			dbVar + `, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))`,
			"if err != nil { return nil, err }",
			"state." + field + " = " + alias + "." + param.Reconstructor.ConstructorName + "(" + dbVar + ")",
		}
	case "http_client":
		return []string{"state." + field + " = &http.Client{Timeout: 30 * time.Second}"}
	case "logger":
		return []string{`state.` + field + ` = log.New(os.Stderr, "", log.LstdFlags)`}
	default:
		return []string{"// missing reconstructor for " + param.Name}
	}
}

func ServerCommandDir(plan *Plan) string {
	return filepath.Dir(plan.ServerPath)
}

const serverTemplate = `package main

import (
{{- range .Imports }}
	{{ if .Alias }}{{ .Alias }} {{ end }}"{{ .Path }}"
{{- end }}
)

type invokeRequest struct {
{{- range .RequestFields }}
	{{ .Name }} {{ .Type }} ` + "`json:\"{{ .JSONName }}\"`" + `
{{- end }}
}

type invokeResponse struct {
{{- if .LocalizedResult }}
	Error *localizedError ` + "`json:\"error,omitempty\"`" + `
{{- else if .HasResult }}
	{{ .ResponseField.Name }} {{ .ResponseField.Type }} ` + "`json:\"{{ .ResponseField.JSONName }}\"`" + `
{{- end }}
}

type localizedError struct {
	Error   string ` + "`json:\"error,omitempty\"`" + `
	Message string ` + "`json:\"message,omitempty\"`" + `
}

type invocationRecord struct {
	ID     uint64         ` + "`json:\"id\"`" + `
	Params invokeRequest  ` + "`json:\"params\"`" + `
	Result invokeResponse ` + "`json:\"result\"`" + `
}

type invocationRecorder struct {
	mu      sync.Mutex
	count   uint64
	records []invocationRecord
}

const invocationHistoryLimit = 100

func newInvocationRecorder() *invocationRecorder {
	return &invocationRecorder{records: make([]invocationRecord, 0, invocationHistoryLimit)}
}

func (r *invocationRecorder) record(params invokeRequest, result invokeResponse) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.count++
	id := r.count
	r.records = append(r.records, invocationRecord{ID: id, Params: params, Result: result})
	if len(r.records) > invocationHistoryLimit {
		copy(r.records, r.records[len(r.records)-invocationHistoryLimit:])
		r.records = r.records[:invocationHistoryLimit]
	}
	return id
}

func (r *invocationRecorder) countSnapshot() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func (r *invocationRecorder) recordsSnapshot() []invocationRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]invocationRecord, len(r.records))
	copy(records, r.records)
	return records
}

type callsResponse struct {
	Count uint64 ` + "`json:\"count\"`" + `
}

type invocationsResponse struct {
	Records []invocationRecord ` + "`json:\"records\"`" + `
}

type serverState struct {
	invocations *invocationRecorder
{{- range .StateFields }}
	{{ .Name }} {{ .Type }}
{{- end }}
}

func initState() (*serverState, error) {
	state := &serverState{invocations: newInvocationRecorder()}
{{- range .StateInitLines }}
	{{ . }}
{{- end }}
	return state, nil
}

func NewHandler(state *serverState) http.Handler {
	if state == nil {
		state = &serverState{}
	}
	if state.invocations == nil {
		state.invocations = newInvocationRecorder()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/calls", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(callsResponse{Count: state.invocations.countSnapshot()}); err != nil {
			log.Printf("encode calls response: %v", err)
		}
	})
	mux.HandleFunc("/invocations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(invocationsResponse{Records: state.invocations.recordsSnapshot()}); err != nil {
			log.Printf("encode invocations response: %v", err)
		}
	})
	mux.HandleFunc("/invoke", invokeHandler(state))
	return mux
}

func invokeHandler(state *serverState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var req invokeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
{{- if .LocalizedResult }}
		result := {{ .CutPackageAlias }}.{{ .GeneratedFunction }}({{ .CallArgs }})
		var resp invokeResponse
		if result != nil {
			var errText string
			if err := result.Error(); err != nil {
				errText = err.Error()
			}
			resp.Error = &localizedError{Error: errText, Message: result.Translate("en_US")}
		}
{{- else if .HasResult }}
		result := {{ .CutPackageAlias }}.{{ .GeneratedFunction }}({{ .CallArgs }})
		resp := invokeResponse{ {{ .ResponseField.Name }}: result }
{{- else }}
		{{ .CutPackageAlias }}.{{ .GeneratedFunction }}({{ .CallArgs }})
		resp := invokeResponse{}
{{- end }}
		invocationID := state.invocations.record(req, resp)
		log.Printf("LIFT_INVOKE service={{ .Plan.ServiceName }} invocation_id=%d", invocationID)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("encode response: %v", err)
		}
	}
}

func main() {
	state, err := initState()
	if err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("MONOLIFT_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	log.Fatal(http.ListenAndServe(addr, NewHandler(state)))
}
`
