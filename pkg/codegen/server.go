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
	Plan                           *Plan
	Imports                        []importSpec
	RequestFields                  []fieldView
	ResponseField                  fieldView
	StateFields                    []fieldView
	StateInitLines                 []string
	StateCloseLines                []string
	RequestValidationLines         []string
	NeedsRootRelativePathValidator bool
	CallArgs                       string
	HasResult                      bool
	HasErrorResult                 bool
	LocalizedResult                bool
	PrimitiveResult                bool
	CutPackageAlias                string
	AdapterFunc                    string
	LocalAdapterCode               string

	// ResultDTO support: when HasDTO is true, the response carries multiple
	// non-error fields packed into a single struct.
	HasDTO    bool
	DTOFields []fieldView
	// DTOCallVars are the LHS variable names for the multi-return call.
	DTOCallVars string
	// DTORespLiteral is the struct literal packing call vars into invokeResponse.
	DTORespLiteral string

	// Receiver support
	HasReceiver         bool
	ReceiverRequestType string // qualified type for invokeRequest field (ReceiverBoundary only)
	ReceiverConstruct   string // construction statement before adapter call (Factory/Zero-pointer)
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
	if plan != nil && plan.AdapterPlan != nil {
		plan = normalizedAdapterPlan(plan)
	}
	imports := []importSpec{
		{Path: "encoding/json"},
		{Path: "log"},
		{Path: "net/http"},
		{Path: "os"},
		{Path: "sync"},
	}
	if plan.CutPoint.PackageName != "main" {
		imports = append(imports, importSpec{Path: plan.CutPoint.PackagePath})
	}
	var localAdapterCode string
	if plan.CutPoint.PackageName == "main" && plan.AdapterPlan != nil {
		if helper, err := buildNormalizedHelper(plan); err == nil {
			imports = append(imports, helper.Imports...)
			localAdapterCode = renderLocalAdapterCode(plan, helper)
		}
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
	if planNeedsMinifluxConfigInit(plan) {
		imports = append(imports, importSpec{Path: "miniflux.app/v2/internal/config"})
		stateInitLines = append(stateInitLines,
			"cfg := config.NewConfigParser()",
			"opts, parseErr := cfg.ParseEnvironmentVariables()",
			"if parseErr != nil { return nil, parseErr }",
			"config.Opts = opts",
		)
	}
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
			stateCloseLines = append(stateCloseLines, serverReconstructorCloseSource(param))
		}
	}
	if receiverParam, ok := reconstructedReceiverParam(plan); ok {
		stateFields = append(stateFields, fieldView{
			Name:          exportedFieldName(receiverParam.Name),
			OriginalName:  receiverParam.Name,
			Type:          receiverParam.QualifiedGoType,
			Reconstructor: receiverParam.Reconstructor,
		})
		for _, raw := range receiverParam.Reconstructor.Imports {
			imports = append(imports, importSpecFromRaw(raw))
		}
		stateInitLines = append(stateInitLines, serverReconstructorInit(receiverParam)...)
		if receiverParam.Reconstructor.CloseSource != "" {
			stateCloseLines = append(stateCloseLines, serverReconstructorCloseSource(receiverParam))
		}
	}
	// Separate non-error results from error results.
	response := fieldView{}
	hasNonErrorResult := false
	hasErrorResult := false
	localized := false
	primitive := false
	var dtoFields []fieldView
	hasDTO := plan.ResultDTO != nil
	for _, result := range plan.Results {
		if result.Codec == CodecError {
			hasErrorResult = true
			continue
		}
		if hasDTO {
			hasNonErrorResult = true
			continue
		}
		if !hasNonErrorResult {
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
			hasNonErrorResult = true
		}
	}
	// Build DTO field views and call/response metadata.
	var dtoCallVars, dtoRespLiteral string
	if hasDTO {
		for _, f := range plan.ResultDTO.Fields {
			dtoFields = append(dtoFields, fieldView{
				Name:         f.Name,
				JSONName:     f.JSONName,
				Type:         f.QualifiedGoType,
				OriginalName: f.OriginalName,
				ZeroValue:    zeroValue(f.GoType),
			})
			if f.QualifiedGoType != f.GoType {
				// May need an import for the qualified type.
				for _, r := range plan.Results {
					if r.Index == f.Index && r.TypePackagePath != "" && r.TypePackagePath != plan.CutPoint.PackagePath {
						imports = append(imports, importSpec{Path: r.TypePackagePath})
					}
				}
			}
		}
		// Build the LHS vars for multi-value call: r0, r1, ..., resultErr
		var callVars []string
		for i := range plan.ResultDTO.Fields {
			callVars = append(callVars, "r"+string(rune('0'+i)))
		}
		if hasErrorResult {
			callVars = append(callVars, "resultErr")
		}
		dtoCallVars = strings.Join(callVars, ", ")
		// Build resp literal: invokeResponse{ Field0: r0, Field1: r1 }
		var litParts []string
		for i, f := range plan.ResultDTO.Fields {
			litParts = append(litParts, f.Name+": r"+string(rune('0'+i)))
		}
		dtoRespLiteral = strings.Join(litParts, ", ")
	}
	baseCallArgs := strings.Join(callArgs(plan.BoundaryParams, plan.ReconstructedParams, "state."), ", ")
	requestValidationLines := rootRelativePathValidationLines(plan)
	if len(requestValidationLines) > 0 {
		imports = append(imports, importSpec{Path: "fmt"})
		imports = append(imports, importSpec{Path: "path/filepath"})
		imports = append(imports, importSpec{Path: "strings"})
	}

	view := serverView{
		Plan:                           plan,
		Imports:                        uniqueImports(imports),
		RequestFields:                  requestFields,
		ResponseField:                  response,
		StateFields:                    stateFields,
		StateInitLines:                 stateInitLines,
		StateCloseLines:                stateCloseLines,
		RequestValidationLines:         requestValidationLines,
		NeedsRootRelativePathValidator: len(requestValidationLines) > 0,
		CallArgs:                       baseCallArgs,
		HasResult:                      hasNonErrorResult,
		HasErrorResult:                 hasErrorResult,
		LocalizedResult:                localized,
		PrimitiveResult:                primitive,
		CutPackageAlias:                plan.CutPoint.PackageName,
		AdapterFunc:                    adapterFuncName(plan.CutPoint.FuncName),
		LocalAdapterCode:               localAdapterCode,
		HasDTO:                         hasDTO,
		DTOFields:                      dtoFields,
		DTOCallVars:                    dtoCallVars,
		DTORespLiteral:                 dtoRespLiteral,
	}

	// Receiver support: compute request field, construction, and call arg.
	if plan.ReceiverParam != nil {
		view.HasReceiver = true
		baseType := strings.TrimPrefix(plan.ReceiverParam.GoType, "*")
		qualifiedBase := plan.CutPoint.PackageName + "." + baseType

		var receiverCallArg string
		switch plan.ReceiverParam.Policy {
		case ReceiverBoundary:
			view.ReceiverRequestType = qualifiedBase
			if plan.ReceiverParam.IsPointer {
				receiverCallArg = "&req.Receiver"
			} else {
				receiverCallArg = "req.Receiver"
			}
		case ReceiverFactory:
			view.ReceiverConstruct = "recv := " + plan.CutPoint.PackageName + "." + plan.ReceiverParam.FactoryFunc + "(" + strings.Join(plan.ReceiverParam.FactoryArgs, ", ") + ")"
			receiverCallArg = "recv"
		case ReceiverReconstructed:
			receiverCallArg = "state.Receiver"
		case ReceiverZero:
			if plan.ReceiverParam.IsPointer {
				view.ReceiverConstruct = "recv := &" + qualifiedBase + "{}"
				receiverCallArg = "recv"
			} else {
				receiverCallArg = qualifiedBase + "{}"
			}
		}

		if view.CallArgs != "" {
			view.CallArgs = receiverCallArg + ", " + view.CallArgs
		} else {
			view.CallArgs = receiverCallArg
		}
	}

	return view
}

func (v serverView) AdapterCall() string {
	if v.LocalAdapterCode != "" {
		return v.AdapterFunc
	}
	return v.CutPackageAlias + "." + v.AdapterFunc
}

// renderLocalAdapterCode inlines the adapter wrapper and normalized helper into
// the extracted service's main package (used when the cut function lives in
// package main and cannot be imported). Any package-level constants the helper
// references are copied in ahead of it, since the cut package is out of scope.
func renderLocalAdapterCode(plan *Plan, helper *normalizedHelper) string {
	transport := normalizedAdapterPlan(plan)
	paramList := adapterParamList(transport.BoundaryParams)
	resultList := computeStubReturnSig(transport.Results)
	var b strings.Builder
	for _, decl := range helper.FreeConsts {
		b.WriteString(decl)
		b.WriteString("\n\n")
	}
	b.WriteString("func " + adapterFuncName(plan.CutPoint.FuncName) + "(" + paramList + ") " + resultList + " {\n\treturn " + normalizedHelperFuncName(plan) + "(" + clientOriginalArgs(fieldsFromParams(transport.BoundaryParams)) + ")\n}\n\n")
	b.WriteString("func " + normalizedHelperFuncName(plan) + "(" + paramList + ") " + resultList + " {\n" + helper.Body + "\n}\n")
	return b.String()
}

func planNeedsMinifluxConfigInit(plan *Plan) bool {
	if plan == nil {
		return false
	}
	for _, param := range plan.ReconstructedParams {
		if param.Reconstructor.ID == "sql_db_wrapper" && param.TypePackagePath == "miniflux.app/v2/internal/storage" {
			return true
		}
	}
	return false
}

func serverReconstructorInit(param ReconstructedParam) []string {
	field := exportedFieldName(param.Name)
	if len(param.Reconstructor.InitLines) > 0 || len(param.Reconstructor.StartupProbeLines) > 0 || len(param.Reconstructor.ConstructorLines) > 0 {
		lines := make([]string, 0, len(param.Reconstructor.InitLines)+len(param.Reconstructor.StartupProbeLines)+len(param.Reconstructor.ConstructorLines))
		lines = append(lines, renderReconstructorLines(field, param.Reconstructor.InitLines)...)
		lines = append(lines, renderReconstructorLines(field, param.Reconstructor.StartupProbeLines)...)
		lines = append(lines, renderReconstructorLines(field, param.Reconstructor.ConstructorLines)...)
		return lines
	}
	dbVar := strings.ToLower(param.Name) + "DB"
	switch param.Reconstructor.ID {
	case "context_background":
		return []string{"state." + field + " = context.Background()"}
	case "discard_logger":
		return []string{"state." + field + " = nil"}
	case "sql_db":
		return []string{
			dbVar + `, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))`,
			"if err != nil { return nil, err }",
			"if err := " + dbVar + ".PingContext(context.Background()); err != nil { _ = " + dbVar + ".Close(); return nil, err }",
			"state." + field + " = " + dbVar,
		}
	case "sql_db_wrapper":
		constructorPkg := reconstructorConstructorPkg(param)
		constructorFunc := reconstructorConstructorFunc(param)
		constructorArgs := reconstructorConstructorArgs(param, dbVar)
		return []string{
			dbVar + `, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))`,
			"if err != nil { return nil, err }",
			"if err := " + dbVar + ".PingContext(context.Background()); err != nil { _ = " + dbVar + ".Close(); return nil, err }",
			"state." + field + " = " + constructorPkg + "." + constructorFunc + "(" + strings.Join(constructorArgs, ", ") + ")",
		}
	case "http_client":
		return []string{"state." + field + " = &http.Client{Timeout: 30 * time.Second}"}
	case "logger":
		return []string{`state.` + field + ` = log.New(os.Stderr, "", log.LstdFlags)`}
	default:
		return []string{"// missing reconstructor for " + param.Name}
	}
}

func serverReconstructorCloseSource(param ReconstructedParam) string {
	field := exportedFieldName(param.Name)
	if strings.Contains(param.Reconstructor.CloseSource, "$") {
		return renderReconstructorLine(field, param.Reconstructor.CloseSource)
	}
	return strings.ReplaceAll(param.Reconstructor.CloseSource, "db", strings.ToLower(param.Name)+"DB")
}

func renderReconstructorLines(field string, lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, renderReconstructorLine(field, line))
	}
	return out
}

func renderReconstructorLine(field, line string) string {
	resourceVar := lowerIdentifier(field)
	replacements := map[string]string{
		"$STATE_FIELD":    field,
		"$RESOURCE_VAR":   resourceVar,
		"$ROOT_VAR":       resourceVar + "Root",
		"$CLEAN_ROOT_VAR": resourceVar + "CleanRoot",
		"$INFO_VAR":       resourceVar + "RootInfo",
	}
	for old, new := range replacements {
		line = strings.ReplaceAll(line, old, new)
	}
	return line
}

func lowerIdentifier(name string) string {
	if name == "" {
		return "resource"
	}
	return strings.ToLower(name[:1]) + name[1:]
}

func rootRelativePathValidationLines(plan *Plan) []string {
	suffixes := rootRelativePathSuffixes(plan)
	if len(suffixes) == 0 {
		return nil
	}
	var lines []string
	seen := map[string]struct{}{}
	for _, param := range plan.BoundaryParams {
		if param.GoType != "string" && param.QualifiedGoType != "string" {
			continue
		}
		if !matchesRootRelativePathSuffix(param, suffixes) {
			continue
		}
		field := exportedFieldName(param.Name)
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		lines = append(lines, `if err := monoliftValidateRootRelativePath("`+param.JSONName+`", req.`+field+`); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }`)
	}
	return lines
}

func rootRelativePathSuffixes(plan *Plan) []string {
	var suffixes []string
	seen := map[string]struct{}{}
	for _, reconstructor := range planReconstructors(plan) {
		for _, suffix := range reconstructor.RootRelativePathSuffixes {
			if suffix == "" {
				continue
			}
			if _, ok := seen[suffix]; ok {
				continue
			}
			seen[suffix] = struct{}{}
			suffixes = append(suffixes, suffix)
		}
	}
	return suffixes
}

func matchesRootRelativePathSuffix(param Param, suffixes []string) bool {
	field := exportedFieldName(param.Name)
	jsonSuffixes := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		jsonSuffixes = append(jsonSuffixes, strings.TrimPrefix(toSnake(suffix), "_"))
		if strings.HasSuffix(param.Name, suffix) || strings.HasSuffix(field, suffix) {
			return true
		}
	}
	for _, suffix := range jsonSuffixes {
		if param.JSONName == suffix || strings.HasSuffix(param.JSONName, "_"+suffix) {
			return true
		}
	}
	return false
}

func reconstructorConstructorPkg(param ReconstructedParam) string {
	pkg := param.Reconstructor.ConstructorPkg
	if pkg == "" {
		pkg = param.Reconstructor.ConstructorPackageAlias
	}
	if pkg == "" {
		pkg = param.Reconstructor.ConstructorPackagePath
	}
	if pkg == "" {
		pkg = param.TypePackagePath
	}
	if strings.Contains(pkg, "/") {
		return packageAlias(pkg)
	}
	return pkg
}

func reconstructorConstructorFunc(param ReconstructedParam) string {
	if param.Reconstructor.ConstructorFunc != "" {
		return param.Reconstructor.ConstructorFunc
	}
	return param.Reconstructor.ConstructorName
}

func reconstructorConstructorArgs(param ReconstructedParam, dbVar string) []string {
	if len(param.Reconstructor.ConstructorArgOrder) == 0 {
		return []string{dbVar}
	}
	args := make([]string, 0, len(param.Reconstructor.ConstructorArgOrder))
	for _, arg := range param.Reconstructor.ConstructorArgOrder {
		switch arg {
		case "db", "sql_db", "*sql.DB":
			args = append(args, dbVar)
		default:
			args = append(args, arg)
		}
	}
	return args
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
{{- if .ReceiverRequestType }}
	Receiver {{ .ReceiverRequestType }} ` + "`json:\"receiver\"`" + `
{{- end }}
{{- range .RequestFields }}
	{{ .Name }} {{ .Type }} ` + "`json:\"{{ .JSONName }}\"`" + `
{{- end }}
}

type invokeResponse struct {
{{- if .LocalizedResult }}
	Error *localizedError ` + "`json:\"error,omitempty\"`" + `
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
{{- if .StateCloseLines }}
	closeFuncs  []func() error
{{- end }}
{{- range .StateFields }}
	{{ .Name }} {{ .Type }}
{{- end }}
}

func initState() (*serverState, error) {
	state := &serverState{invocations: newInvocationRecorder()}
{{- range .StateInitLines }}
	{{ . }}
{{- end }}
{{- range .StateCloseLines }}
	state.closeFuncs = append(state.closeFuncs, func() error { return {{ . }} })
{{- end }}
	return state, nil
}

{{ if .StateCloseLines }}
func (state *serverState) Close() {
	if state == nil {
		return
	}
	for i := len(state.closeFuncs) - 1; i >= 0; i-- {
		if err := state.closeFuncs[i](); err != nil {
			log.Printf("close state resource: %v", err)
		}
	}
}
{{ end }}

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

{{ if .NeedsRootRelativePathValidator }}
func monoliftValidateRootRelativePath(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s must be root-relative and non-empty", name)
	}
	if filepath.IsAbs(value) {
		return fmt.Errorf("%s must be root-relative", name)
	}
	clean := filepath.Clean(value)
	for _, part := range strings.Split(filepath.ToSlash(value), "/") {
		if part == ".." {
			return fmt.Errorf("%s must not contain .. traversal", name)
		}
	}
	if clean == "." {
		return fmt.Errorf("%s must not escape the durable root", name)
	}
	return nil
}
{{ end }}

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
{{- range .RequestValidationLines }}
		{{ . }}
{{- end }}
{{- if .ReceiverConstruct }}
		{{ .ReceiverConstruct }}
{{- end }}
{{- if .LocalizedResult }}
		result := {{ .AdapterCall }}({{ .CallArgs }})
		var resp invokeResponse
		if result != nil {
			var errText string
			if err := result.Error(); err != nil {
				errText = err.Error()
			}
			resp.Error = &localizedError{Error: errText, Message: result.Translate("en_US")}
		}
{{- else if .HasDTO }}
		{{ .DTOCallVars }} := {{ .AdapterCall }}({{ .CallArgs }})
		resp := invokeResponse{ {{ .DTORespLiteral }} }
{{- if .HasErrorResult }}
		if resultErr != nil {
			resp.Error = resultErr.Error()
		}
{{- end }}
{{- else if .HasErrorResult }}
{{- if .HasResult }}
		result, resultErr := {{ .AdapterCall }}({{ .CallArgs }})
		resp := invokeResponse{ {{ .ResponseField.Name }}: result }
{{- else }}
		resultErr := {{ .AdapterCall }}({{ .CallArgs }})
		resp := invokeResponse{}
{{- end }}
		if resultErr != nil {
			resp.Error = resultErr.Error()
		}
{{- else if .HasResult }}
		result := {{ .AdapterCall }}({{ .CallArgs }})
		resp := invokeResponse{ {{ .ResponseField.Name }}: result }
{{- else }}
		{{ .AdapterCall }}({{ .CallArgs }})
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

{{ .LocalAdapterCode }}

func main() {
	state, err := initState()
	if err != nil {
		log.Fatal(err)
	}
{{- if .StateCloseLines }}
	defer state.Close()
{{- end }}
	addr := os.Getenv("MONOLIFT_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
{{- if .StateCloseLines }}
	if err := http.ListenAndServe(addr, NewHandler(state)); err != nil {
		log.Printf("listen and serve: %v", err)
	}
{{- else }}
	log.Fatal(http.ListenAndServe(addr, NewHandler(state)))
{{- end }}
}
`
