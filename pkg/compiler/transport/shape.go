package transport

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

type Shape string

const (
	ShapeHTTPHandler        Shape = "http-handler"
	ShapeChannelConsumer    Shape = "channel-consumer"
	ShapeBuilderChain       Shape = "builder-chain"
	ShapeCtxRequestResponse Shape = "ctx-request-response"
	ShapeMultiDomainArgs    Shape = "multi-domain-args"
	ShapeNoResponse         Shape = "no-response"
	ShapeUnsupported        Shape = "unsupported"
)

type Classification struct {
	Admission        string
	Operation        reportv2.SymbolIdentity
	Properties       []reportv2.PropertyEvidence
	Shape            Shape
	DefaultTransport string
	Evidence         []string
}

type Result struct {
	Root         Classification
	PerOperation []Classification
	Diagnostics  []extract.Diagnostic
}

func Classify(loaded *extract.LoadedModule, program *ssa.Program, root reportv2.Root) (Result, error) {
	if loaded == nil {
		return Result{}, fmt.Errorf("shape: loaded module is nil")
	}
	if program == nil {
		return Result{}, fmt.Errorf("shape: program is nil")
	}
	lift, err := liftability.Classify(loaded, program, root)
	if err != nil {
		return Result{}, err
	}
	return classifyWithLiftability(loaded, program, root, toExtractLiftabilityResult(lift))
}

func classifyWithLiftability(loaded *extract.LoadedModule, program *ssa.Program, root reportv2.Root, lift extract.LiftabilityResult) (Result, error) {
	ops, err := exposedOperations(loaded, root)
	if err != nil {
		return Result{}, err
	}

	perOperation := make([]Classification, 0, len(ops))
	diagnostics := append([]extract.Diagnostic(nil), lift.Diagnostics...)
	liftByOperation := indexLiftabilityByOperation(lift.PerOperation)
	for _, op := range ops {
		handle, err := resolveOperation(loaded, program, op)
		if err != nil {
			return Result{}, err
		}
		classification := classifyOperation(loaded, handle, liftByOperation[identityKey(handle.identity)])
		perOperation = append(perOperation, classification)
	}

	sort.Slice(perOperation, func(i, j int) bool {
		return identityKey(perOperation[i].Operation) < identityKey(perOperation[j].Operation)
	})
	sortDiagnostics(diagnostics)

	rootClassification, rootDiagnostics := aggregateRoot(loaded, root, lift.Root, perOperation)
	diagnostics = append(diagnostics, rootDiagnostics...)
	sortDiagnostics(diagnostics)
	return Result{
		Root:         rootClassification,
		PerOperation: perOperation,
		Diagnostics:  diagnostics,
	}, nil
}

func ForExtract(loaded *extract.LoadedModule, program *ssa.Program, root reportv2.Root, lift extract.LiftabilityResult) (extract.ShapeResult, error) {
	result, err := classifyWithLiftability(loaded, program, root, lift)
	if err != nil {
		return extract.ShapeResult{}, err
	}
	return toExtractShapeResult(result), nil
}

func ValidatePragmaOptions(loaded *extract.LoadedModule, root reportv2.Root, _ extract.LiftabilityResult, classified extract.ShapeResult) []extract.Diagnostic {
	diagnostics := make([]extract.Diagnostic, 0, 4)
	diagnostics = append(diagnostics, validateTransportAgainstShape(loaded, classified.Root)...)
	diagnostics = append(diagnostics, validateStateAffinityKey(loaded)...)
	diagnostics = append(diagnostics, validateMethodsAgainstRoot(loaded, root)...)
	sortDiagnostics(diagnostics)
	return diagnostics
}

func aggregateRoot(loaded *extract.LoadedModule, root reportv2.Root, liftRoot extract.LiftabilityClassification, perOperation []Classification) (Classification, []extract.Diagnostic) {
	rootClassification := Classification{
		Admission:        liftRoot.Admission,
		Operation:        root.Identity,
		Properties:       append([]reportv2.PropertyEvidence(nil), liftRoot.Properties...),
		Shape:            ShapeUnsupported,
		DefaultTransport: "",
		Evidence:         []string{},
	}
	if len(perOperation) == 0 {
		rootClassification.Evidence = []string{"no exposed operations resolved for root"}
		return rootClassification, nil
	}
	if len(perOperation) == 1 {
		rootClassification.Shape = perOperation[0].Shape
		rootClassification.DefaultTransport = perOperation[0].DefaultTransport
		rootClassification.Evidence = append([]string(nil), perOperation[0].Evidence...)
		return rootClassification, nil
	}

	shapes := make([]Shape, 0, len(perOperation))
	for _, operation := range perOperation {
		shapes = append(shapes, operation.Shape)
	}
	if allSameShape(shapes) {
		rootClassification.Shape = shapes[0]
		rootClassification.DefaultTransport = perOperation[0].DefaultTransport
		rootClassification.Evidence = []string{fmt.Sprintf("aggregated %d operations with the same canonical shape", len(perOperation))}
		return rootClassification, nil
	}

	if allDomainShapes(shapes) {
		rootClassification.Shape = aggregateDomainShape(shapes)
		rootClassification.DefaultTransport = defaultTransportForShape(rootClassification.Shape)
		rootClassification.Evidence = []string{fmt.Sprintf("aggregated %d domain-shape operations", len(perOperation))}
		return rootClassification, nil
	}

	if containsShape(shapes, ShapeHTTPHandler) && containsAnyShape(shapes, ShapeCtxRequestResponse, ShapeMultiDomainArgs, ShapeNoResponse) {
		// TODO(SPRINT-0008-mixed-surface): support mixed handler + domain struct surfaces.
		diag := diagnostic(loaded.RootPragma.Span, "MLV2_STRUCT_SURFACE_UNSUPPORTED", "mixed handler and domain operations on one struct surface are unsupported", "AS-STRUCT-2")
		rootClassification.Evidence = []string{"mixed handler and domain operations on one struct surface"}
		return rootClassification, []extract.Diagnostic{diag}
	}

	if loaded.RootPragma.Surface == extract.SurfaceStruct && loaded.RootPragma.Options["methods"] == "" && containsAnyShape(shapes, ShapeUnsupported, ShapeBuilderChain) {
		diag := diagnostic(loaded.RootPragma.Span, "MLV2_STRUCT_SURFACE_UNSUPPORTED", "struct surface exposes an unsupported canonical shape", "AS-STRUCT-2")
		rootClassification.Evidence = []string{"struct surface includes an unsupported or builder-chain operation"}
		return rootClassification, []extract.Diagnostic{diag}
	}

	rootClassification.Evidence = []string{"root surface spans multiple incompatible canonical shapes"}
	return rootClassification, nil
}

func toExtractShapeResult(result Result) extract.ShapeResult {
	return extract.ShapeResult{
		Root:         toExtractClassification(result.Root),
		PerOperation: toExtractClassifications(result.PerOperation),
		Diagnostics:  append([]extract.Diagnostic(nil), result.Diagnostics...),
	}
}

func toExtractClassifications(classifications []Classification) []extract.ShapeClassification {
	out := make([]extract.ShapeClassification, 0, len(classifications))
	for _, classification := range classifications {
		out = append(out, toExtractClassification(classification))
	}
	return out
}

func toExtractClassification(classification Classification) extract.ShapeClassification {
	return extract.ShapeClassification{
		Operation:        classification.Operation,
		Shape:            string(classification.Shape),
		DefaultTransport: classification.DefaultTransport,
		Evidence:         append([]string(nil), classification.Evidence...),
	}
}

func toExtractLiftabilityResult(result liftability.Result) extract.LiftabilityResult {
	return extract.LiftabilityResult{
		Root:         toExtractLiftabilityClassification(result.Root),
		PerOperation: toExtractLiftabilityClassifications(result.PerOperation),
		Diagnostics:  append([]extract.Diagnostic(nil), result.Diagnostics...),
	}
}

func toExtractLiftabilityClassifications(items []liftability.Classification) []extract.LiftabilityClassification {
	out := make([]extract.LiftabilityClassification, 0, len(items))
	for _, item := range items {
		out = append(out, toExtractLiftabilityClassification(item))
	}
	return out
}

func toExtractLiftabilityClassification(item liftability.Classification) extract.LiftabilityClassification {
	out := make([]reportv2.PropertyEvidence, 0, len(item.Properties))
	for _, property := range item.Properties {
		out = append(out, reportv2.PropertyEvidence{
			PropertyID: string(property.PropertyID),
			Subject:    property.Subject,
			Verdict:    string(property.Verdict),
			Source:     string(property.Source),
			Detail:     property.Detail,
		})
	}
	return extract.LiftabilityClassification{
		Operation:   item.Operation,
		Admission:   string(item.Admission),
		Properties:  out,
		RefusalCode: item.RefusalCode,
	}
}

func indexLiftabilityByOperation(items []extract.LiftabilityClassification) map[string]extract.LiftabilityClassification {
	out := make(map[string]extract.LiftabilityClassification, len(items))
	for _, item := range items {
		out[identityKey(item.Operation)] = item
	}
	return out
}

func propertyVerdict(properties []reportv2.PropertyEvidence, propertyID string) string {
	for _, property := range properties {
		if property.PropertyID == propertyID {
			return property.Verdict
		}
	}
	return ""
}

func propertyPresent(properties []reportv2.PropertyEvidence, propertyID string) bool {
	return propertyVerdict(properties, propertyID) != ""
}

// site:begin canonical-shapes-classifier
func classifyOperation(loaded *extract.LoadedModule, handle operationHandle, lift extract.LiftabilityClassification) Classification {
	classification := Classification{
		Admission:        lift.Admission,
		Operation:        handle.identity,
		Properties:       append([]reportv2.PropertyEvidence(nil), lift.Properties...),
		Shape:            ShapeUnsupported,
		DefaultTransport: "",
		Evidence:         []string{},
	}
	selection := Select(buildSelectionInput(loaded, handle, lift))
	return newClassification(classification, selection.Shape, selection.DefaultTransport, selection.Evidence)
}

// site:end canonical-shapes-classifier

func emitOperationDiagnostics(loaded *extract.LoadedModule) bool {
	return loaded.RootPragma.Surface == extract.SurfaceFunction ||
		loaded.RootPragma.Surface == extract.SurfaceMethod ||
		loaded.RootPragma.Options["methods"] != ""
}

type operationHandle struct {
	identity  reportv2.SymbolIdentity
	signature *types.Signature
	function  *ssa.Function
}

func resolveOperation(loaded *extract.LoadedModule, program *ssa.Program, op reportv2.SymbolIdentity) (operationHandle, error) {
	identity := op
	if op.Kind == "method" && !strings.Contains(op.ObjectName, ".") {
		resolved, err := resolveDirectMethodIdentity(loaded, op)
		if err != nil {
			return operationHandle{}, err
		}
		identity = resolved
	}

	switch identity.Kind {
	case "function":
		obj := loaded.RootPkg.Types.Scope().Lookup(identity.ObjectName)
		if obj == nil {
			return operationHandle{}, fmt.Errorf("shape: root function %q not found", identity.ObjectName)
		}
		fn, ok := obj.(*types.Func)
		if !ok {
			return operationHandle{}, fmt.Errorf("shape: root function %q resolved to %T", identity.ObjectName, obj)
		}
		signature, ok := fn.Type().(*types.Signature)
		if !ok {
			return operationHandle{}, fmt.Errorf("shape: function %q does not have a signature", identity.ObjectName)
		}
		return operationHandle{
			identity:  identity,
			signature: signature,
			function:  lookupSSAFunction(program, loaded.RootPkg.Types, identity.ObjectName),
		}, nil
	case "method":
		selection, err := lookupMethodSelection(loaded, identity.ObjectName)
		if err != nil {
			return operationHandle{}, err
		}
		fn, ok := selection.Obj().(*types.Func)
		if !ok {
			return operationHandle{}, fmt.Errorf("shape: method %q resolved to %T", identity.ObjectName, selection.Obj())
		}
		signature, ok := fn.Type().(*types.Signature)
		if !ok {
			return operationHandle{}, fmt.Errorf("shape: method %q does not have a signature", identity.ObjectName)
		}
		return operationHandle{
			identity:  identity,
			signature: signature,
			function:  program.MethodValue(selection),
		}, nil
	default:
		return operationHandle{}, fmt.Errorf("shape: unsupported operation kind %q", identity.Kind)
	}
}

func exposedOperations(loaded *extract.LoadedModule, root reportv2.Root) ([]reportv2.SymbolIdentity, error) {
	if len(root.ExposedOperations) > 0 {
		out := append([]reportv2.SymbolIdentity(nil), root.ExposedOperations...)
		sort.Slice(out, func(i, j int) bool {
			return identityKey(out[i]) < identityKey(out[j])
		})
		return out, nil
	}
	if root.Identity.Kind == "method" {
		op, err := resolveDirectMethodIdentity(loaded, root.Identity)
		if err != nil {
			return nil, err
		}
		return []reportv2.SymbolIdentity{op}, nil
	}
	if root.Identity.Kind == "function" {
		return []reportv2.SymbolIdentity{root.Identity}, nil
	}
	return []reportv2.SymbolIdentity{}, nil
}

func resolveDirectMethodIdentity(loaded *extract.LoadedModule, identity reportv2.SymbolIdentity) (reportv2.SymbolIdentity, error) {
	funcDecl := findRootMethodDecl(loaded)
	if funcDecl == nil || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
		return reportv2.SymbolIdentity{}, fmt.Errorf("shape: could not resolve receiver for root method %q", identity.ObjectName)
	}
	recvType := receiverTypeName(funcDecl.Recv.List[0].Type)
	if recvType == "" {
		return reportv2.SymbolIdentity{}, fmt.Errorf("shape: unsupported receiver expression for root method %q", identity.ObjectName)
	}

	objName := recvType + "." + identity.ObjectName
	if _, ok := funcDecl.Recv.List[0].Type.(*ast.StarExpr); ok {
		objName = "(*" + recvType + ")." + identity.ObjectName
	}
	identity.ObjectName = objName
	return identity, nil
}

func findRootMethodDecl(loaded *extract.LoadedModule) *ast.FuncDecl {
	for _, file := range loaded.RootPkg.Syntax {
		filePath := loaded.Fset.Position(file.Pos()).Filename
		if !samePath(filePath, loaded.RootFile) {
			continue
		}
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Doc == nil || funcDecl.Name == nil || funcDecl.Name.Name != loaded.RootPragma.DeclName {
				continue
			}
			for _, comment := range funcDecl.Doc.List {
				pos := loaded.Fset.Position(comment.Slash)
				if samePath(pos.Filename, loaded.RootFile) && pos.Line == loaded.RootPragma.Span.Line {
					return funcDecl
				}
			}
		}
	}
	return nil
}

func receiverTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		if ident, ok := typed.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func isHTTPHandler(loaded *extract.LoadedModule, handle operationHandle) ([]string, bool) {
	if matchesNetHTTPHandler(loaded, handle.signature) {
		return []string{"signature matches net/http handler"}, true
	}
	if matchesCaddyMiddlewareHandler(loaded, handle.signature) {
		return []string{"signature matches caddyhttp.MiddlewareHandler"}, true
	}
	return nil, false
}

func matchesNetHTTPHandler(loaded *extract.LoadedModule, signature *types.Signature) bool {
	if signature == nil || signature.Params().Len() != 2 || signature.Results().Len() != 0 {
		return false
	}
	responseWriterType, requestPtrType, ok := netHTTPTypes(loaded)
	if !ok {
		return false
	}
	return types.Identical(signature.Params().At(0).Type(), responseWriterType) &&
		types.Identical(signature.Params().At(1).Type(), requestPtrType)
}

func matchesCaddyMiddlewareHandler(loaded *extract.LoadedModule, signature *types.Signature) bool {
	caddySignature, ok := caddyServeHTTPSignature(loaded)
	if !ok {
		return false
	}
	return types.Identical(signature, caddySignature)
}

func isChannelConsumer(handle operationHandle) ([]string, bool) {
	if handle.function == nil || handle.signature == nil {
		return nil, false
	}
	if channelCrossesBoundary(handle.signature) {
		return nil, false
	}

	receives := false
	backEdge := false
	for _, block := range handle.function.Blocks {
		for _, succ := range block.Succs {
			if succ != nil && succ.Index <= block.Index {
				backEdge = true
			}
		}
		for _, instr := range block.Instrs {
			switch typed := instr.(type) {
			case *ssa.UnOp:
				if typed.Op == token.ARROW {
					receives = true
				}
			case *ssa.Select:
				for _, state := range typed.States {
					if state.Dir == types.RecvOnly || state.Dir == types.SendRecv {
						receives = true
					}
				}
			}
		}
	}
	if !receives || !backEdge {
		return nil, false
	}
	return []string{"ssa contains channel receive within a loop without channel-typed boundary values"}, true
}

func isBuilderChain(signature *types.Signature) ([]string, bool) {
	if signature == nil || signature.Recv() == nil || signature.Results().Len() == 0 {
		return nil, false
	}
	recvType := signature.Recv().Type()
	resultType := signature.Results().At(0).Type()
	if types.AssignableTo(resultType, recvType) || types.AssignableTo(recvType, resultType) {
		return []string{"first result is assignable to the receiver type"}, true
	}
	return nil, false
}

func isCtxRequestResponse(loaded *extract.LoadedModule, signature *types.Signature) ([]string, bool) {
	if signature == nil || signature.Params().Len() != 2 || signature.Results().Len() != 2 {
		return nil, false
	}
	contextType, ok := contextContextType(loaded)
	if !ok {
		return nil, false
	}
	if !types.Identical(signature.Params().At(0).Type(), contextType) {
		return nil, false
	}
	if !isErrorType(signature.Results().At(1).Type()) {
		return nil, false
	}
	return []string{"signature matches func(context.Context, T) (U, error)"}, true
}

func isMultiDomainArgs(loaded *extract.LoadedModule, signature *types.Signature) ([]string, bool) {
	if signature == nil || signature.Params().Len() < 3 || signature.Results().Len() == 0 {
		return nil, false
	}
	contextType, ok := contextContextType(loaded)
	if !ok {
		return nil, false
	}
	if !types.Identical(signature.Params().At(0).Type(), contextType) {
		return nil, false
	}
	if !isErrorType(signature.Results().At(signature.Results().Len() - 1).Type()) {
		return nil, false
	}
	return []string{"signature matches context-first multi-domain argument form"}, true
}

func isNoResponse(loaded *extract.LoadedModule, handle operationHandle) ([]string, *extract.Diagnostic, bool) {
	if handle.signature == nil {
		return nil, nil, false
	}
	switch handle.signature.Results().Len() {
	case 0:
		diag := diagnostic(loaded.RootPragma.Span, "MLV2_NO_ERROR_CHANNEL", "no-response roots must return an error channel when dispatched remotely", "TA-SHAPE-1", "SS-WALDO-2")
		return []string{"signature returns no values"}, &diag, true
	case 1:
		if isErrorType(handle.signature.Results().At(0).Type()) {
			return []string{"signature returns only error"}, nil, true
		}
	}
	return nil, nil, false
}

func validateTransportAgainstShape(loaded *extract.LoadedModule, root extract.ShapeClassification) []extract.Diagnostic {
	switch loaded.RootPragma.Options["transport"] {
	case "":
		return nil
	case "grpc":
		return []extract.Diagnostic{diagnostic(loaded.RootPragma.Span, "MLV2_TRANSPORT_RESERVED", "transport=grpc is reserved and not implemented in v2", "TA-GRPC-1")}
	case "handler":
		if root.Shape != string(ShapeHTTPHandler) {
			return []extract.Diagnostic{diagnostic(loaded.RootPragma.Span, "MLV2_SHAPE_UNSUPPORTED", "transport=handler requires an http-handler root shape", "TA-HANDLER-1")}
		}
	}
	return nil
}

func validateStateAffinityKey(loaded *extract.LoadedModule) []extract.Diagnostic {
	if loaded.RootPragma.Options["state"] == "affinity" && loaded.RootPragma.Options["affinity"] == "" {
		return []extract.Diagnostic{diagnostic(loaded.RootPragma.Span, "MLV2_SESSION_AFFINITY_UNAVAILABLE", "state=affinity requires an affinity= key at the lift point", "SS-LIFT-6")}
	}
	return nil
}

func validateMethodsAgainstRoot(loaded *extract.LoadedModule, root reportv2.Root) []extract.Diagnostic {
	methods := splitMethods(loaded.RootPragma.Options["methods"])
	if len(methods) == 0 {
		return nil
	}
	actual := map[string]bool{}
	for _, operation := range root.ExposedOperations {
		if method := methodName(operation.ObjectName); method != "" {
			actual[method] = true
		}
	}
	var diagnostics []extract.Diagnostic
	for _, method := range methods {
		if actual[method] {
			continue
		}
		switch loaded.RootPragma.Surface {
		case extract.SurfaceStruct:
			diagnostics = append(diagnostics, diagnostic(loaded.RootPragma.Span, "MLV2_STRUCT_SURFACE_UNSUPPORTED", fmt.Sprintf("methods=%s names a method that is not exposed on the struct root", method), "AS-STRUCT-2"))
		case extract.SurfaceInterface:
			diagnostics = append(diagnostics, diagnostic(loaded.RootPragma.Span, "MLV2_SHAPE_UNSUPPORTED", fmt.Sprintf("methods=%s names a method that is not exposed on the interface root", method), "TA-SHAPE-1", "TA-REFUSE-1", "AS-FUNC-2"))
		}
	}
	return diagnostics
}

func hasBoundaryType(signature *types.Signature, match func(types.Type) bool) bool {
	for i := 0; i < signature.Params().Len(); i++ {
		if match(signature.Params().At(i).Type()) {
			return true
		}
	}
	for i := 0; i < signature.Results().Len(); i++ {
		if match(signature.Results().At(i).Type()) {
			return true
		}
	}
	return false
}

func isChannelType(typ types.Type) bool {
	_, ok := typ.(*types.Chan)
	return ok
}

func channelCrossesBoundary(signature *types.Signature) bool {
	return hasBoundaryType(signature, isChannelType)
}

func defaultTransportForShape(shape Shape) string {
	switch shape {
	case ShapeHTTPHandler:
		return "handler"
	case ShapeCtxRequestResponse, ShapeMultiDomainArgs, ShapeNoResponse:
		return "http-json"
	default:
		return ""
	}
}

func selectDefaultTransport(loaded *extract.LoadedModule, shape Shape) string {
	transport := loaded.RootPragma.Options["transport"]
	switch transport {
	case "handler":
		if shape == ShapeHTTPHandler {
			return "handler"
		}
	case "http-json":
		if shape == ShapeCtxRequestResponse || shape == ShapeMultiDomainArgs || shape == ShapeNoResponse {
			return "http-json"
		}
	}
	return defaultTransportForShape(shape)
}

func newClassification(base Classification, shape Shape, defaultTransport string, evidence []string) Classification {
	stableEvidence := append([]string(nil), evidence...)
	sort.Strings(stableEvidence)
	base.Shape = shape
	base.DefaultTransport = defaultTransport
	base.Evidence = stableEvidence
	return base
}

func diagnostic(span extract.Span, code, message string, ruleIDs ...string) extract.Diagnostic {
	return extract.Diagnostic{
		Code:     code,
		Severity: extract.SeverityError,
		Message:  message,
		Span:     span,
		RuleIDs:  append([]string(nil), ruleIDs...),
	}
}

func sortDiagnostics(diags []extract.Diagnostic) {
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Code != diags[j].Code {
			return diags[i].Code < diags[j].Code
		}
		if diags[i].Span.Filename != diags[j].Span.Filename {
			return diags[i].Span.Filename < diags[j].Span.Filename
		}
		if diags[i].Span.Line != diags[j].Span.Line {
			return diags[i].Span.Line < diags[j].Span.Line
		}
		return diags[i].Message < diags[j].Message
	})
}

func allSameShape(shapes []Shape) bool {
	if len(shapes) == 0 {
		return false
	}
	first := shapes[0]
	for _, shape := range shapes[1:] {
		if shape != first {
			return false
		}
	}
	return true
}

func allDomainShapes(shapes []Shape) bool {
	if len(shapes) == 0 {
		return false
	}
	for _, shape := range shapes {
		if shape != ShapeCtxRequestResponse && shape != ShapeMultiDomainArgs {
			return false
		}
	}
	return true
}

func aggregateDomainShape(shapes []Shape) Shape {
	for _, shape := range shapes {
		if shape == ShapeMultiDomainArgs {
			return ShapeMultiDomainArgs
		}
	}
	return ShapeCtxRequestResponse
}

func containsShape(shapes []Shape, target Shape) bool {
	for _, shape := range shapes {
		if shape == target {
			return true
		}
	}
	return false
}

func containsAnyShape(shapes []Shape, targets ...Shape) bool {
	for _, target := range targets {
		if containsShape(shapes, target) {
			return true
		}
	}
	return false
}

func identityKey(identity reportv2.SymbolIdentity) string {
	return identity.ModulePath + "|" + identity.PackagePath + "|" + identity.Kind + "|" + identity.ObjectName
}

func splitMethods(value string) []string {
	if value == "" {
		return nil
	}
	raw := strings.Split(value, ",")
	out := make([]string, 0, len(raw))
	for _, method := range raw {
		method = strings.TrimSpace(method)
		if method != "" {
			out = append(out, method)
		}
	}
	return out
}

func methodName(objectName string) string {
	_, method, _, ok := parseMethodObjectName(objectName)
	if !ok {
		return ""
	}
	return method
}

func lookupSSAFunction(program *ssa.Program, pkg *types.Package, name string) *ssa.Function {
	ssaPkg := program.Package(pkg)
	if ssaPkg == nil {
		return nil
	}
	member := ssaPkg.Members[name]
	fn, _ := member.(*ssa.Function)
	return fn
}

func lookupMethodSelection(loaded *extract.LoadedModule, objectName string) (*types.Selection, error) {
	typeName, methodName, pointerRecv, ok := parseMethodObjectName(objectName)
	if !ok {
		return nil, fmt.Errorf("shape: could not parse method identity %q", objectName)
	}
	obj := loaded.RootPkg.Types.Scope().Lookup(typeName)
	if obj == nil {
		return nil, fmt.Errorf("shape: receiver type %q not found", typeName)
	}
	typeNameObj, ok := obj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("shape: receiver %q resolved to %T", typeName, obj)
	}
	named, ok := typeNameObj.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("shape: receiver %q is not a named type", typeName)
	}
	if iface, ok := named.Underlying().(*types.Interface); ok {
		iface.Complete()
		for i := 0; i < iface.NumMethods(); i++ {
			method := iface.Method(i)
			if method.Name() == methodName {
				return types.NewMethodSet(named).Lookup(loaded.RootPkg.Types, methodName), nil
			}
		}
		return nil, fmt.Errorf("shape: interface method %q not found on %s", methodName, typeName)
	}
	if pointerRecv {
		if selection := types.NewMethodSet(types.NewPointer(named)).Lookup(loaded.RootPkg.Types, methodName); selection != nil {
			return selection, nil
		}
	}
	if selection := types.NewMethodSet(named).Lookup(loaded.RootPkg.Types, methodName); selection != nil {
		return selection, nil
	}
	if selection := types.NewMethodSet(types.NewPointer(named)).Lookup(loaded.RootPkg.Types, methodName); selection != nil {
		return selection, nil
	}
	return nil, fmt.Errorf("shape: method %q not found on %s", methodName, typeName)
}

func parseMethodObjectName(name string) (typeName, methodName string, pointerRecv, ok bool) {
	if strings.HasPrefix(name, "(*") {
		right := strings.Index(name, ").")
		if right < 0 {
			return "", "", false, false
		}
		return name[2:right], name[right+2:], true, true
	}
	left := strings.LastIndex(name, ".")
	if left < 0 {
		return "", "", false, false
	}
	return name[:left], name[left+1:], false, true
}

func contextContextType(loaded *extract.LoadedModule) (types.Type, bool) {
	pkg := findImportedTypesPackage(loaded.RootPkg, "context")
	if pkg == nil {
		return nil, false
	}
	obj := pkg.Scope().Lookup("Context")
	if obj == nil {
		return nil, false
	}
	return obj.Type(), true
}

func netHTTPTypes(loaded *extract.LoadedModule) (types.Type, types.Type, bool) {
	pkg := findImportedTypesPackage(loaded.RootPkg, "net/http")
	if pkg == nil {
		return nil, nil, false
	}
	responseWriter := pkg.Scope().Lookup("ResponseWriter")
	request := pkg.Scope().Lookup("Request")
	if responseWriter == nil || request == nil {
		return nil, nil, false
	}
	return responseWriter.Type(), types.NewPointer(request.Type()), true
}

func caddyServeHTTPSignature(loaded *extract.LoadedModule) (*types.Signature, bool) {
	pkg := findImportedTypesPackage(loaded.RootPkg, "github.com/caddyserver/caddy/v2/modules/caddyhttp")
	if pkg == nil {
		return nil, false
	}
	obj := pkg.Scope().Lookup("MiddlewareHandler")
	if obj == nil {
		return nil, false
	}
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return nil, false
	}
	iface, ok := named.Underlying().(*types.Interface)
	if !ok {
		return nil, false
	}
	iface.Complete()
	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		if method.Name() == "ServeHTTP" {
			signature, ok := method.Type().(*types.Signature)
			return signature, ok
		}
	}
	return nil, false
}

func isErrorType(typ types.Type) bool {
	errorType := types.Universe.Lookup("error")
	return errorType != nil && types.Identical(typ, errorType.Type())
}

func findImportedTypesPackage(root *packages.Package, target string) *types.Package {
	seen := map[string]bool{}
	var walk func(pkg *packages.Package) *types.Package
	walk = func(pkg *packages.Package) *types.Package {
		if pkg == nil {
			return nil
		}
		key := pkg.PkgPath
		if key == "" {
			key = pkg.ID
		}
		if seen[key] {
			return nil
		}
		seen[key] = true
		if pkg.Types != nil && pkg.Types.Path() == target {
			return pkg.Types
		}
		for _, imported := range pkg.Imports {
			if found := walk(imported); found != nil {
				return found
			}
		}
		return nil
	}
	return walk(root)
}

func samePath(left, right string) bool {
	return filepath.Clean(strings.TrimSpace(left)) == filepath.Clean(strings.TrimSpace(right))
}
