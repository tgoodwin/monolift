package extract

import (
	constantpkg "go/constant"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

const codeReflectionDispatch = "MLV2_REFLECTION_DISPATCH"
const codeClosureUnbounded = "MLV2_CLOSURE_UNBOUNDED"
const codeDynamicPlugin = "MLV2_DYNAMIC_PLUGIN"

func detectReflectionDispatch(loaded *loadedModule, root reportv2.Root, funcs []*ssa.Function) []Diagnostic {
	if root.RegistryKey != nil {
		return nil
	}

	diagnostics := map[string]Diagnostic{}
	for _, fn := range funcs {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(ssa.CallInstruction)
				if !ok {
					continue
				}
				common := call.Common()
				callee := common.StaticCallee()
				if callee == nil || callee.Package() == nil || callee.Package().Pkg == nil {
					continue
				}
				if callee.Package().Pkg.Path() != "reflect" {
					continue
				}
				name := functionObjectName(callee)
				if name != "Value.Call" && name != "Value.CallSlice" && name != "Value.MethodByName" && name != "Type.MethodByName" {
					continue
				}

				span := functionSpan(loaded, instr.Pos())
				key := codeReflectionDispatch + "|" + span.FileRelativePath + "|" + name
				diagnostics[key] = Diagnostic{
					Code:       codeReflectionDispatch,
					Severity:   SeverityError,
					Message:    "reflection-driven dispatch is not statically bounded",
					Span:       spanToDiagnostic(span),
					Suggestion: "replace reflection-driven dispatch with direct calls or an explicit registry-keyed site",
				}
			}
		}
	}

	return sortedDiagnostics(diagnostics)
}

func detectUnsafeBoundary(loaded *loadedModule, funcs []*ssa.Function) []Diagnostic {
	diagnostics := map[string]Diagnostic{}
	for _, fn := range funcs {
		if fn == nil || !functionSignatureTouchesUnsafe(fn) {
			continue
		}
		span := functionSpan(loaded, fn.Pos())
		key := codeClosureUnbounded + "|" + span.FileRelativePath + "|" + functionObjectName(fn)
		diagnostics[key] = Diagnostic{
			Code:       codeClosureUnbounded,
			Severity:   SeverityError,
			Message:    "unsafe.Pointer crosses the extraction boundary",
			Span:       spanToDiagnostic(span),
			Suggestion: "remove unsafe.Pointer from lifted function boundaries or externalize the boundary before lifting",
		}
	}
	return sortedDiagnostics(diagnostics)
}

func detectDynamicPluginLoads(loaded *loadedModule, root reportv2.Root, funcs []*ssa.Function) []Diagnostic {
	if root.RegistryKey != nil {
		return nil
	}

	diagnostics := map[string]Diagnostic{}
	for _, fn := range funcs {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(ssa.CallInstruction)
				if !ok {
					continue
				}
				common := call.Common()
				callee := common.StaticCallee()
				if callee == nil || callee.Package() == nil || callee.Package().Pkg == nil || callee.Package().Pkg.Path() != "plugin" {
					continue
				}
				name := functionObjectName(callee)
				if name != "Open" && name != "(*Plugin).Lookup" && name != "Plugin.Lookup" {
					continue
				}
				if lastArgIsConstantString(common.Args) {
					continue
				}

				span := functionSpan(loaded, instr.Pos())
				baseKey := span.FileRelativePath + "|" + name
				diagnostics[codeDynamicPlugin+"|"+baseKey] = Diagnostic{
					Code:       codeDynamicPlugin,
					Severity:   SeverityError,
					Message:    "dynamic plugin loading is not statically bounded",
					Span:       spanToDiagnostic(span),
					Suggestion: "replace dynamic plugin resolution with a statically visible registry-keyed implementation set",
				}
				diagnostics[codeClosureUnbounded+"|"+baseKey] = Diagnostic{
					Code:       codeClosureUnbounded,
					Severity:   SeverityError,
					Message:    "dynamic plugin loading leaves the closure frontier unbounded",
					Span:       spanToDiagnostic(span),
					Suggestion: "replace dynamic plugin resolution with a statically visible registry-keyed implementation set",
				}
			}
		}
	}
	return sortedDiagnostics(diagnostics)
}

func applyRefusalMetadata(report *reportv2.Report, diagnostics []Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	report.Pragma.Options["verdict"] = "refuse-blocking"
	report.Pruning.Bounded = false
}

func sortedDiagnostics(in map[string]Diagnostic) []Diagnostic {
	out := make([]Diagnostic, 0, len(in))
	for _, diagnostic := range in {
		out = append(out, diagnostic)
	}
	sortDiagnostics(out)
	return out
}

func sortDiagnostics(out []Diagnostic) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		if out[i].Span.Filename != out[j].Span.Filename {
			return out[i].Span.Filename < out[j].Span.Filename
		}
		if out[i].Span.Line != out[j].Span.Line {
			return out[i].Span.Line < out[j].Span.Line
		}
		return out[i].Message < out[j].Message
	})
}

func spanToDiagnostic(span reportv2.SourceSpan) Span {
	return Span{
		Filename: span.FileRelativePath,
		Line:     nonZero(span.LineStart, 1),
		EndLine:  nonZero(span.LineEnd, nonZero(span.LineStart, 1)),
	}
}

func nonZero(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func functionSignatureTouchesUnsafe(fn *ssa.Function) bool {
	if fn == nil || fn.Signature == nil {
		return false
	}
	if recv := fn.Signature.Recv(); recv != nil && typeContainsUnsafePointer(recv.Type()) {
		return true
	}
	params := fn.Signature.Params()
	for i := 0; i < params.Len(); i++ {
		if typeContainsUnsafePointer(params.At(i).Type()) {
			return true
		}
	}
	results := fn.Signature.Results()
	for i := 0; i < results.Len(); i++ {
		if typeContainsUnsafePointer(results.At(i).Type()) {
			return true
		}
	}
	return false
}

func lastArgIsConstantString(args []ssa.Value) bool {
	if len(args) == 0 {
		return false
	}
	constant, ok := args[len(args)-1].(*ssa.Const)
	return ok && constant.Value != nil && constant.Value.Kind() == constantpkg.String
}

func typeContainsUnsafePointer(typ types.Type) bool {
	switch t := typ.(type) {
	case nil:
		return false
	case *types.Basic:
		return t.Kind() == types.UnsafePointer
	case *types.Named:
		return typeContainsUnsafePointer(t.Underlying())
	case *types.Alias:
		return typeContainsUnsafePointer(types.Unalias(t))
	default:
		return false
	}
}
