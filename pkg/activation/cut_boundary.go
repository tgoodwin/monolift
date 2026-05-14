package activation

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

func classifyBoundaryData(fn *ssa.Function) (BoundaryDataClass, []string) {
	if fn == nil || fn.Signature == nil {
		return BoundaryInfeasible, []string{"function is missing SSA signature"}
	}

	signature := fn.Signature
	worst := Trivial
	explanations := []string{}

	// The receiver is NOT boundary data — it lives on the remote side and is
	// classified under state reconstruction cost (classifyState). Only
	// parameters and return values cross the network.

	params := signature.Params()
	for i := 0; i < params.Len(); i++ {
		param := params.At(i)
		variadic := signature.Variadic() && i == params.Len()-1
		label := tupleValueLabel("param", i, param)
		class, reason := classifyBoundaryValue(label, param.Type(), variadic)
		worst = worseBoundaryClass(worst, class)
		explanations = append(explanations, reason)
	}

	results := signature.Results()
	for i := 0; i < results.Len(); i++ {
		result := results.At(i)
		label := tupleValueLabel("result", i, result)
		class, reason := classifyBoundaryValue(label, result.Type(), false)
		worst = worseBoundaryClass(worst, class)
		explanations = append(explanations, reason)
	}

	if len(explanations) == 0 {
		explanations = append(explanations, "no receiver, parameters, or returns: Trivial")
	}
	return worst, explanations
}

func classifyBoundaryValue(label string, typ types.Type, variadic bool) (BoundaryDataClass, string) {
	if typ == nil {
		return BoundaryInfeasible, fmt.Sprintf("%s <nil>: %s (missing type)", label, BoundaryInfeasible)
	}
	if variadic {
		if slice, ok := types.Unalias(typ).(*types.Slice); ok {
			typ = slice.Elem()
		}
	}
	if typ == nil {
		return BoundaryInfeasible, fmt.Sprintf("%s <nil>: %s (missing variadic element type)", label, BoundaryInfeasible)
	}
	class, reason := classifyBoundaryType(typ, map[types.Type]bool{})
	return class, fmt.Sprintf("%s %s: %s (%s)", label, cutTypeString(typ), class, reason)
}

func classifyBoundaryType(typ types.Type, seen map[types.Type]bool) (BoundaryDataClass, string) {
	if typ == nil {
		return BoundaryInfeasible, "missing type"
	}
	typ = types.Unalias(typ)
	if seen[typ] {
		return Serializable, "recursive type already seen"
	}
	seen[typ] = true
	defer delete(seen, typ)

	if class, reason, ok := knownBoundaryType(typ); ok {
		return class, reason
	}

	switch t := typ.(type) {
	case *types.Basic:
		if t.Info()&types.IsString != 0 ||
			t.Info()&types.IsBoolean != 0 ||
			t.Info()&types.IsInteger != 0 ||
			t.Info()&types.IsFloat != 0 ||
			t.Info()&types.IsComplex != 0 {
			return Trivial, "primitive value"
		}
		if t.Kind() == types.UnsafePointer {
			return BoundaryInfeasible, "unsafe pointer cannot cross a network boundary"
		}
		return Serializable, "basic value"
	case *types.Signature:
		return BoundaryInfeasible, "function value cannot cross a network boundary"
	case *types.Chan:
		return BoundaryInfeasible, "channel at cut point means cut is too shallow (ADR-0028)"
	case *types.Pointer:
		if class, reason, ok := knownBoundaryType(t.Elem()); ok {
			return class, reason
		}
		class, reason := classifyBoundaryType(t.Elem(), seen)
		if class == Trivial {
			return Serializable, "pointer to trivial data is serializable with nil/reference encoding"
		}
		return class, "pointer element: " + reason
	case *types.Slice:
		if isByteType(t.Elem()) {
			return Trivial, "byte slice"
		}
		class, reason := classifyBoundaryType(t.Elem(), seen)
		return worseBoundaryClass(Serializable, class), "slice element: " + reason
	case *types.Array:
		if isByteType(t.Elem()) {
			return Trivial, "byte array"
		}
		class, reason := classifyBoundaryType(t.Elem(), seen)
		return worseBoundaryClass(Serializable, class), "array element: " + reason
	case *types.Map:
		keyClass, keyReason := classifyBoundaryType(t.Key(), seen)
		elemClass, elemReason := classifyBoundaryType(t.Elem(), seen)
		class := worseBoundaryClass(Serializable, worseBoundaryClass(keyClass, elemClass))
		return class, "map key: " + keyReason + "; map value: " + elemReason
	case *types.Struct:
		return classifyBoundaryStruct(t, seen)
	case *types.Interface:
		if t == nil {
			return Serializable, "nil interface treated as serializable"
		}
		return classifyBoundaryInterface(t, seen)
	case *types.Named:
		if class, reason, ok := knownBoundaryType(t); ok {
			return class, reason
		}
		underlying := t.Underlying()
		if underlying == nil {
			return Serializable, "named type with nil underlying treated as serializable"
		}
		class, reason := classifyBoundaryType(underlying, seen)
		return class, "named underlying type: " + reason
	case *types.TypeParam:
		return Serializable, "type parameter assumed serializable"
	default:
		return Serializable, "unknown type shape treated as serializable"
	}
}

func classifyBoundaryStruct(strct *types.Struct, seen map[types.Type]bool) (BoundaryDataClass, string) {
	if strct.NumFields() == 0 {
		return Trivial, "empty struct"
	}

	worst := Trivial
	allExportedTrivial := true
	reasons := make([]string, 0, strct.NumFields())
	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		if isSyncPrimitive(field.Type()) {
			reasons = append(reasons, fmt.Sprintf("%s=skip (sync primitive, zero-initializable on remote side)", field.Name()))
			continue
		}
		class, reason := classifyBoundaryType(field.Type(), seen)
		if !field.Exported() && !field.Embedded() && class == Trivial {
			class = Serializable
			reason = "unexported field requires explicit serialization"
		}
		if class != Trivial || (!field.Exported() && !field.Embedded()) {
			allExportedTrivial = false
		}
		worst = worseBoundaryClass(worst, class)
		reasons = append(reasons, fmt.Sprintf("%s=%s", field.Name(), reason))
	}

	if allExportedTrivial {
		return Trivial, "struct of exported trivial fields"
	}
	return worseBoundaryClass(Serializable, worst), "struct fields: " + strings.Join(reasons, "; ")
}

func classifyBoundaryInterface(iface *types.Interface, seen map[types.Type]bool) (class BoundaryDataClass, reason string) {
	if iface == nil {
		return Serializable, "nil interface"
	}
	defer func() {
		if r := recover(); r != nil {
			class = Serializable
			reason = fmt.Sprintf("interface inspection panicked: %v", r)
		}
	}()
	iface = iface.Complete()
	if iface.NumMethods() == 0 {
		return Serializable, "empty interface"
	}

	worst := Serializable
	reasons := make([]string, 0, iface.NumMethods())
	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		signature, ok := method.Type().(*types.Signature)
		if !ok {
			continue
		}
		if proxyLikeInterfaceMethod(method.Name(), signature) {
			worst = worseBoundaryClass(worst, BoundaryInfeasible)
			reasons = append(reasons, fmt.Sprintf("%s=%s (streaming method per ADR-0028)", method.Name(), BoundaryInfeasible))
			continue
		}
		class := classifyBoundaryMethodSignature(signature, seen)
		worst = worseBoundaryClass(worst, class)
		reasons = append(reasons, fmt.Sprintf("%s=%s", method.Name(), class))
	}
	return worst, "interface methods: " + strings.Join(reasons, "; ")
}

func proxyLikeInterfaceMethod(name string, signature *types.Signature) bool {
	if signature == nil {
		return false
	}
	switch name {
	case "Read", "Write":
		params := signature.Params()
		results := signature.Results()
		if params.Len() == 1 && isByteSlice(params.At(0).Type()) && results.Len() == 2 && typeImplementsError(results.At(1).Type()) {
			return true
		}
	}
	return false
}

func classifyBoundaryMethodSignature(signature *types.Signature, seen map[types.Type]bool) BoundaryDataClass {
	worst := Serializable
	params := signature.Params()
	for i := 0; i < params.Len(); i++ {
		typ := params.At(i).Type()
		if signature.Variadic() && i == params.Len()-1 {
			if slice, ok := types.Unalias(typ).(*types.Slice); ok {
				typ = slice.Elem()
			}
		}
		class, _ := classifyBoundaryType(typ, seen)
		worst = worseBoundaryClass(worst, class)
	}
	results := signature.Results()
	for i := 0; i < results.Len(); i++ {
		class, _ := classifyBoundaryType(results.At(i).Type(), seen)
		worst = worseBoundaryClass(worst, class)
	}
	return worst
}

func knownBoundaryType(typ types.Type) (BoundaryDataClass, string, bool) {
	if typ == nil {
		return BoundaryInfeasible, "missing type", true
	}
	named, pointer := namedType(typ)
	if named == nil || named.Obj() == nil {
		return "", "", false
	}
	obj := named.Obj()
	pkgPath := ""
	if obj.Pkg() != nil {
		pkgPath = obj.Pkg().Path()
	}
	name := obj.Name()

	switch pkgPath {
	case "context":
		if name == "Context" {
			return Serializable, "context metadata is serializable", true
		}
	case "io":
		if name == "Writer" || name == "WriteCloser" {
			return BoundaryInfeasible, "streaming IO output at cut point means cut is too shallow (ADR-0028)", true
		}
		if name == "Reader" || name == "ReadCloser" || name == "ReadSeeker" {
			return Serializable, "streaming reader can be serialized as bounded byte payload", true
		}
	case "net/http":
		switch name {
		case "ResponseWriter":
			return BoundaryInfeasible, "http.ResponseWriter at cut point means cut is too shallow (ADR-0028)", true
		case "Request":
			return Serializable, "http.Request can be serialized for RPC", true
		case "Client":
			return Reconstructible, "HTTP client can be reconstructed from config", true
		}
	case "database/sql":
		if pointer && name == "DB" {
			return Reconstructible, "database pool can be reconstructed from config", true
		}
	case "log":
		if pointer && name == "Logger" {
			return Reconstructible, "logger can be reconstructed from config", true
		}
	case "html/template", "text/template":
		if pointer && name == "Template" {
			return Reconstructible, "template can be reconstructed from parsed config", true
		}
	case "os":
		switch name {
		case "Process":
			return BoundaryInfeasible, "process handle cannot cross a network boundary", true
		case "File":
			return BoundaryInfeasible, "file handle at cut point means cut is too shallow (ADR-0028)", true
		}
	case "sync":
		switch name {
		case "Mutex", "RWMutex", "WaitGroup", "Cond", "Once", "Pool", "Map":
			return BoundaryInfeasible, "sync primitive cannot cross a network boundary", true
		}
	case "time":
		if name == "Timer" || name == "Ticker" {
			return BoundaryInfeasible, "runtime lifecycle handle cannot cross a network boundary", true
		}
	}

	if frameworkContextType(pkgPath, name) {
		return Reconstructible, "framework context can be reconstructed from request data", true
	}
	if pointer && configBackedTypeName(name) {
		return Reconstructible, "config-backed service type can be reconstructed", true
	}
	return "", "", false
}

func frameworkContextType(pkgPath, name string) bool {
	switch {
	case pkgPath == "github.com/labstack/echo/v4" && name == "Context":
		return true
	case pkgPath == "github.com/labstack/echo" && name == "Context":
		return true
	case strings.HasSuffix(pkgPath, "/context") && (name == "Context" || name == "APIContext" || name == "PrivateContext" || name == "ResponseWriter"):
		return true
	case strings.HasSuffix(pkgPath, "/request") && name == "CTX":
		return true
	case pkgPath == "github.com/spf13/cobra" && name == "Command":
		return true
	case strings.HasSuffix(pkgPath, "/core") && (name == "App" || name == "RequestEvent" || name == "Record"):
		return true
	}
	return false
}

func tupleValueLabel(prefix string, index int, value *types.Var) string {
	if value == nil || value.Name() == "" {
		return fmt.Sprintf("%s[%d]", prefix, index)
	}
	return fmt.Sprintf("%s %s", prefix, value.Name())
}

func worseBoundaryClass(a, b BoundaryDataClass) BoundaryDataClass {
	if boundaryClassRank(b) > boundaryClassRank(a) {
		return b
	}
	return a
}

func boundaryClassRank(class BoundaryDataClass) int {
	switch class {
	case Trivial:
		return 0
	case Serializable:
		return 1
	case Reconstructible:
		return 2
	case ProxyRequired:
		return 3
	case BoundaryInfeasible:
		return 4
	default:
		return 4
	}
}

func cutTypeString(typ types.Type) string {
	if typ == nil {
		return "<nil>"
	}
	return types.TypeString(typ, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Path()
	})
}

func namedType(typ types.Type) (*types.Named, bool) {
	typ = types.Unalias(typ)
	if pointer, ok := typ.(*types.Pointer); ok {
		named, _ := namedType(pointer.Elem())
		return named, true
	}
	named, ok := typ.(*types.Named)
	return named, ok
}

func isByteType(typ types.Type) bool {
	basic, ok := types.Unalias(typ).(*types.Basic)
	return ok && basic.Kind() == types.Uint8
}

func isByteSlice(typ types.Type) bool {
	slice, ok := types.Unalias(typ).(*types.Slice)
	return ok && isByteType(slice.Elem())
}

func configBackedTypeName(name string) bool {
	lower := strings.ToLower(name)
	for _, token := range []string{"client", "db", "database", "store", "repository", "repo", "mailer", "emailer", "queue", "publisher", "producer"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

