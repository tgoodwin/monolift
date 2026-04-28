package surface

import (
	"go/types"
	"sort"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/ssa"
)

type SurfaceCategory string

const (
	SurfaceCall    SurfaceCategory = "Call"
	SurfaceSession SurfaceCategory = "Session"
)

type WireProtocol string

const (
	WireProtocolHTTPJSON    WireProtocol = "httpjson"
	WireProtocolStreamProxy WireProtocol = "streamproxy"
)

type EntryPoint struct {
	Identity reportv2.SymbolIdentity
	Function *ssa.Function
	Category SurfaceCategory
	Protocol WireProtocol
	Evidence  []string
}

type RegionSurface struct {
	Category    SurfaceCategory
	WireProtocol WireProtocol
	EntryPoints  []EntryPoint
	Refusals     []Refusal
}

type Refusal struct {
	Code     string
	Message  string
	Subject  reportv2.SymbolIdentity
	Evidence []string
}

const (
	DiagnosticAsyncUnsupported = "MLV2_SURFACE_ASYNC_UNSUPPORTED"
	DiagnosticMixed           = "MLV2_SURFACE_MIXED"
)

func Derive(root reportv2.Root, reachable []*ssa.Function) (RegionSurface, error) {
	entryFns := entryPointFunctions(root, reachable)
	if len(entryFns) == 0 {
		return RegionSurface{
			Category:    SurfaceCall,
			WireProtocol: WireProtocolHTTPJSON,
			EntryPoints: []EntryPoint{{
				Identity: root.Identity,
				Category: SurfaceCall,
				Protocol: WireProtocolHTTPJSON,
				Evidence:  []string{"no function-shaped entry point resolved; retaining legacy call transport"},
			}},
		}, nil
	}
	var out RegionSurface
	categories := map[SurfaceCategory]bool{}
	for _, fn := range entryFns {
		entry := EntryPoint{
			Identity: identityForFunction(root.Identity.ModulePath, fn),
			Function: fn,
		}
		if hasChannelBoundary(fn.Signature) {
			out.Refusals = append(out.Refusals, Refusal{
				Code:     DiagnosticAsyncUnsupported,
				Message:  "entry point passes channels across the region boundary",
				Subject:  entry.Identity,
				Evidence: []string{fn.String()},
			})
			continue
		}
		if exposesSession(fn) {
			entry.Category = SurfaceSession
			entry.Protocol = WireProtocolStreamProxy
			entry.Evidence = []string{"session-capable call reachable from entry point"}
		} else {
			entry.Category = SurfaceCall
			entry.Protocol = WireProtocolHTTPJSON
			entry.Evidence = []string{"function-shaped marshalable boundary"}
		}
		categories[entry.Category] = true
		out.EntryPoints = append(out.EntryPoints, entry)
	}
	sort.Slice(out.EntryPoints, func(i, j int) bool {
		return identityKey(out.EntryPoints[i].Identity) < identityKey(out.EntryPoints[j].Identity)
	})
	sort.Slice(out.Refusals, func(i, j int) bool {
		return out.Refusals[i].Code+identityKey(out.Refusals[i].Subject) < out.Refusals[j].Code+identityKey(out.Refusals[j].Subject)
	})
	if len(categories) > 1 {
		out.Refusals = append(out.Refusals, Refusal{
			Code:    DiagnosticMixed,
			Message: "region entry points derive mixed surface categories",
		})
	}
	if categories[SurfaceSession] {
		out.Category = SurfaceSession
		out.WireProtocol = WireProtocolStreamProxy
	} else {
		out.Category = SurfaceCall
		out.WireProtocol = WireProtocolHTTPJSON
	}
	return out, nil
}

func entryPointFunctions(root reportv2.Root, reachable []*ssa.Function) []*ssa.Function {
	want := root.ExposedOperations
	if len(want) == 0 {
		want = []reportv2.SymbolIdentity{root.Identity}
	}
	seen := map[*ssa.Function]bool{}
	var out []*ssa.Function
	for _, symbol := range want {
		for _, fn := range reachable {
			if fn == nil || seen[fn] || fn.Package() == nil || fn.Package().Pkg == nil {
				continue
			}
			if fn.Package().Pkg.Path() == symbol.PackagePath && functionObjectName(fn) == symbol.ObjectName {
				seen[fn] = true
				out = append(out, fn)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func hasChannelBoundary(sig *types.Signature) bool {
	if sig == nil {
		return false
	}
	for _, tuple := range []*types.Tuple{sig.Params(), sig.Results()} {
		for i := 0; tuple != nil && i < tuple.Len(); i++ {
			if containsChannel(tuple.At(i).Type()) {
				return true
			}
		}
	}
	return false
}

func containsChannel(typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Chan:
		return true
	case *types.Pointer:
		return containsChannel(t.Elem())
	case *types.Slice:
		return containsChannel(t.Elem())
	case *types.Array:
		return containsChannel(t.Elem())
	case *types.Map:
		return containsChannel(t.Key()) || containsChannel(t.Elem())
	case *types.Named:
		return containsChannel(t.Underlying())
	}
	return false
}

func exposesSession(fn *ssa.Function) bool {
	if fn == nil {
		return false
	}
	if typeMentionsSession(fn.Signature) {
		return true
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			common := call.Common()
			if common == nil {
				continue
			}
			if callee := common.StaticCallee(); callee != nil && functionIsSessionPrimitive(callee) {
				return true
			}
			if common.IsInvoke() && strings.Contains(common.Method.Name(), "Hijack") {
				return true
			}
		}
	}
	return false
}

func typeMentionsSession(sig *types.Signature) bool {
	if sig == nil {
		return false
	}
	for _, tuple := range []*types.Tuple{sig.Params(), sig.Results()} {
		for i := 0; tuple != nil && i < tuple.Len(); i++ {
			text := tuple.At(i).Type().String()
			if strings.Contains(text, "net.Conn") || strings.Contains(text, "http.Hijacker") {
				return true
			}
		}
	}
	return false
}

func functionIsSessionPrimitive(fn *ssa.Function) bool {
	if fn == nil {
		return false
	}
	name := fn.Name()
	if name == "Hijack" || name == "Upgrade" {
		return true
	}
	if strings.Contains(fn.String(), "websocket") && name == "Upgrade" {
		return true
	}
	return false
}

func identityForFunction(modulePath string, fn *ssa.Function) reportv2.SymbolIdentity {
	identity := reportv2.SymbolIdentity{ModulePath: modulePath}
	if fn == nil {
		return identity
	}
	if fn.Package() != nil && fn.Package().Pkg != nil {
		identity.PackagePath = fn.Package().Pkg.Path()
	}
	identity.ObjectName = functionObjectName(fn)
	if fn.Signature != nil && fn.Signature.Recv() != nil {
		identity.Kind = "method"
	} else {
		identity.Kind = "function"
	}
	return identity
}

func functionObjectName(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	if fn.Signature != nil && fn.Signature.Recv() != nil {
		return fn.Name()
	}
	if strings.Contains(fn.Name(), "$") {
		return strings.Split(fn.Name(), "$")[0]
	}
	return fn.Name()
}

func identityKey(identity reportv2.SymbolIdentity) string {
	return identity.PackagePath + "." + identity.ObjectName
}
