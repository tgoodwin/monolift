package extract

import (
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"github.com/tgoodwin/monolift/pkg/compiler/surface"
)

// Analyze is the compiler-owned v2 extraction seam for SPRINT-0006.
// Callers must pass parser output directly; this package does not re-parse source comments.
// It loads the annotated root's module and builds SSA with InstantiateGenerics so
// downstream CHA/RTA callgraph analysis sees the compiled, callgraph-ready IR.
func Analyze(req Request) (Result, error) {
	loaded, err := loadModule(req)
	if err != nil {
		return Result{}, err
	}
	built, err := buildProgram(loaded)
	if err != nil {
		return Result{}, err
	}
	report := buildSeedReport(loaded)
	var liftabilityResult LiftabilityResult
	if registeredLiftabilityAnalyzer != nil {
		liftabilityResult, err = registeredLiftabilityAnalyzer(loaded, built.Program, report.Root)
		if err != nil {
			return Result{}, err
		}
		report.Root.Admission = liftabilityResult.Root.Admission
		report.Root.Properties = append([]reportv2.PropertyEvidence(nil), liftabilityResult.Root.Properties...)
	}
	var shapeResult ShapeResult
	if registeredShapeClassifier != nil {
		shapeResult, err = registeredShapeClassifier(loaded, built.Program, report.Root, liftabilityResult)
		if err != nil {
			return Result{}, err
		}
		report.Root.Shape = shapeResult.Root.Shape
		report.Root.DefaultTransport = shapeResult.Root.DefaultTransport
	}
	diagnostics := []Diagnostic{}
	if registeredShapeClassifier != nil {
		// The registered shape classifier already folds liftability diagnostics into
		// ShapeResult.Diagnostics, so re-appending liftabilityResult.Diagnostics here
		// would duplicate the same liftability-origin findings at the extract seam.
		diagnostics = append(diagnostics, shapeResult.Diagnostics...)
	} else {
		diagnostics = append(diagnostics, liftabilityResult.Diagnostics...)
	}
	if registeredShapeValidator != nil {
		diagnostics = append(diagnostics, registeredShapeValidator(loaded, report.Root, liftabilityResult, shapeResult)...)
	}
	closure := buildRegionClosure(loaded, built, report.Root)
	report.Closure = closure.Closure
	report.ExternalDeps = closure.ExternalDeps
	report.Analysis.PrecisionTriggers = closure.PrecisionTriggers
	if registeredSeamDetector != nil {
		seams, seamErr := registeredSeamDetector(loaded, built.Program, closure.ReachableByRoot)
		if seamErr != nil {
			return Result{}, seamErr
		}
		report.Seams = seams
	}
	var archetypeClassification *ArchetypeClassification
	if registeredStateInferer != nil {
		stateResult, stateErr := registeredStateInferer(loaded, built.Program, closure.ReachableFuncs, report.Root, &loaded.RootPragma)
		if stateErr != nil {
			return Result{}, stateErr
		}
		report.State = stateResult.Items
		if stateResult.Classification != nil {
			archetypeClassification = stateResult.Classification
			applyArchetypeClassification(&report.Root, stateResult.Classification)
		}
		diagnostics = append(diagnostics, stateResult.Diagnostics...)
		if len(stateResult.PrecisionTriggers) > 0 {
			report.Analysis.PrecisionTriggers = append(report.Analysis.PrecisionTriggers, stateResult.PrecisionTriggers...)
			sort.Strings(report.Analysis.PrecisionTriggers)
		}
	}
	var regionSurface surface.RegionSurface
	if registeredSurfaceDeriver != nil {
		regionSurface, err = registeredSurfaceDeriver(report.Root, closure.ReachableFuncs)
		if err != nil {
			return Result{}, err
		}
		for _, refusal := range regionSurface.Refusals {
			diagnostics = append(diagnostics, Diagnostic{
				Code:     refusal.Code,
				Severity: SeverityError,
				Message:  refusal.Message,
			})
		}
	}
	report.Adapters = deriveAdapters(report.Root, shapeResult, archetypeClassification)
	diagnostics = append(diagnostics, detectReflectionDispatch(loaded, report.Root, closure.ReachableFuncs)...)
	diagnostics = append(diagnostics, detectUnsafeBoundary(loaded, closure.ReachableFuncs)...)
	diagnostics = append(diagnostics, detectDynamicPluginLoads(loaded, report.Root, closure.ReachableFuncs)...)
	sortDiagnostics(diagnostics)
	applyRefusalMetadata(&report, diagnostics)
	return Result{Report: report, Diagnostics: diagnostics, Surface: regionSurface}, nil
}

func buildSeedReport(loaded *loadedModule) reportv2.Report {
	pragmaOptions := cloneMap(loaded.RootPragma.Options)
	if _, ok := pragmaOptions["verdict"]; !ok {
		pragmaOptions["verdict"] = "accept"
	}

	root := resolveRoot(loaded)
	report := reportv2.Report{
		SchemaVersion: reportv2.SchemaVersion,
		BuildConfig: reportv2.BuildConfig{
			GOOS:               loaded.GOOS,
			GOARCH:             loaded.GOARCH,
			CGOEnabled:         loaded.CGOEnabled,
			BuildTags:          append([]string(nil), loaded.BuildTags...),
			ModuleRoot:         displayModuleRoot(loaded.ModuleRoot),
			WorkspaceMode:      "single-module",
			Tests:              false,
			DependencyManifest: []reportv2.Dependency{},
		},
		Analysis: reportv2.Analysis{
			Algorithm:         "ssa-cha+rta",
			PrecisionTriggers: []string{},
			Deterministic:     true,
		},
		Pragma: reportv2.Pragma{
			Name:    loaded.RootPragma.Name,
			Surface: string(loaded.RootPragma.Surface),
			Span:    reportSpan(loaded.RootPragma.Span, loaded.ModuleRoot),
			Options: pragmaOptions,
		},
		Root: root,
		Closure: reportv2.Closure{
			IncludedSymbols: []reportv2.SymbolEntry{},
			ExcludedSymbols: []reportv2.SymbolEntry{},
			WiringPaths:     []reportv2.WiringPath{},
		},
		State:        []reportv2.StateItem{},
		Adapters:     []reportv2.Adapter{},
		ExternalDeps: []reportv2.ExternalDep{},
		Pruning: reportv2.Pruning{
			Bounded:  true,
			Frontier: []reportv2.SymbolEntry{},
		},
		Diagnostics: []reportv2.Diagnostic{},
	}
	return report
}

func reportSpan(span Span, moduleRoot string) reportv2.SourceSpan {
	relative := span.Filename
	if moduleRoot != "" {
		if rel, err := filepath.Rel(moduleRoot, span.Filename); err == nil && rel != "" && rel != "." {
			relative = filepath.ToSlash(rel)
		}
	}
	lineStart := span.Line
	if lineStart == 0 {
		lineStart = 1
	}
	lineEnd := span.EndLine
	if lineEnd == 0 {
		lineEnd = lineStart
	}
	return reportv2.SourceSpan{
		FileRelativePath: filepath.ToSlash(relative),
		ByteOffsetStart:  0,
		ByteOffsetEnd:    1,
		LineStart:        lineStart,
		LineEnd:          lineEnd,
	}
}

func rootKindForSurface(surface Surface) string {
	switch surface {
	case SurfaceInterface:
		return "interface"
	case SurfaceMethod:
		return "method"
	case SurfaceStruct:
		return "type"
	default:
		return "function"
	}
}

func resolveRoot(loaded *loadedModule) reportv2.Root {
	modulePath := loaded.RootPkg.PkgPath
	if loaded.RootPkg.Module != nil && loaded.RootPkg.Module.Path != "" {
		modulePath = loaded.RootPkg.Module.Path
	}

	root := reportv2.Root{
		Identity: reportv2.SymbolIdentity{
			ModulePath:  modulePath,
			PackagePath: loaded.RootPkg.PkgPath,
			ObjectName:  loaded.RootPragma.DeclName,
			Kind:        rootKindForSurface(loaded.RootPragma.Surface),
		},
		Properties:        []reportv2.PropertyEvidence{},
		ExposedOperations: resolveExposedOperations(loaded, modulePath),
	}
	if registry := loaded.RootPragma.Options["registry"]; registry != "" {
		root.RegistryKey = &registry
	}
	return root
}

func resolveExposedOperations(loaded *loadedModule, modulePath string) []reportv2.SymbolIdentity {
	methods := splitMethodList(loaded.RootPragma.Options["methods"])

	obj := loaded.RootPkg.Types.Scope().Lookup(loaded.RootPragma.DeclName)
	if obj == nil {
		return []reportv2.SymbolIdentity{}
	}

	switch loaded.RootPragma.Surface {
	case SurfaceStruct:
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			return []reportv2.SymbolIdentity{}
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			return []reportv2.SymbolIdentity{}
		}
		if len(methods) == 0 {
			return exportedStructMethodSymbols(loaded.RootPkg.Types, modulePath, loaded.RootPkg.PkgPath, loaded.RootPragma.DeclName, named)
		}
		return selectedStructMethodSymbols(loaded.RootPkg.Types, modulePath, loaded.RootPkg.PkgPath, loaded.RootPragma.DeclName, named, methods)
	case SurfaceInterface:
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			return []reportv2.SymbolIdentity{}
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			return []reportv2.SymbolIdentity{}
		}
		iface, ok := named.Underlying().(*types.Interface)
		if !ok {
			return []reportv2.SymbolIdentity{}
		}
		if len(methods) == 0 {
			return interfaceMethodSymbols(modulePath, loaded.RootPkg.PkgPath, loaded.RootPragma.DeclName, iface)
		}
		return selectedInterfaceMethodSymbols(modulePath, loaded.RootPkg.PkgPath, loaded.RootPragma.DeclName, iface, methods)
	}
	return []reportv2.SymbolIdentity{}
}

func splitMethodList(value string) []string {
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

func resolveStructMethodName(pkg *types.Package, named *types.Named, typeName, method string) string {
	if selection := types.NewMethodSet(types.NewPointer(named)).Lookup(pkg, method); selection != nil {
		return "(*" + typeName + ")." + method
	}
	if selection := types.NewMethodSet(named).Lookup(pkg, method); selection != nil {
		return typeName + "." + method
	}
	return ""
}

func interfaceHasMethod(iface *types.Interface, method string) bool {
	iface.Complete()
	for i := 0; i < iface.NumMethods(); i++ {
		if iface.Method(i).Name() == method {
			return true
		}
	}
	return false
}

func exportedStructMethodSymbols(pkg *types.Package, modulePath, packagePath, typeName string, named *types.Named) []reportv2.SymbolIdentity {
	methodSet := types.NewMethodSet(types.NewPointer(named))
	out := make([]reportv2.SymbolIdentity, 0, methodSet.Len())
	for i := 0; i < methodSet.Len(); i++ {
		selection := methodSet.At(i)
		if selection == nil || selection.Obj() == nil || !selection.Obj().Exported() {
			continue
		}
		out = append(out, reportv2.SymbolIdentity{
			ModulePath:  modulePath,
			PackagePath: packagePath,
			ObjectName:  resolveStructMethodName(pkg, named, typeName, selection.Obj().Name()),
			Kind:        "method",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ObjectName < out[j].ObjectName
	})
	return out
}

func selectedStructMethodSymbols(pkg *types.Package, modulePath, packagePath, typeName string, named *types.Named, methods []string) []reportv2.SymbolIdentity {
	out := make([]reportv2.SymbolIdentity, 0, len(methods))
	for _, method := range methods {
		objectName := resolveStructMethodName(pkg, named, typeName, method)
		if objectName == "" {
			continue
		}
		out = append(out, reportv2.SymbolIdentity{
			ModulePath:  modulePath,
			PackagePath: packagePath,
			ObjectName:  objectName,
			Kind:        "method",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ObjectName < out[j].ObjectName
	})
	return out
}

func interfaceMethodSymbols(modulePath, packagePath, typeName string, iface *types.Interface) []reportv2.SymbolIdentity {
	iface.Complete()
	out := make([]reportv2.SymbolIdentity, 0, iface.NumMethods())
	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		if method == nil || !method.Exported() {
			continue
		}
		out = append(out, reportv2.SymbolIdentity{
			ModulePath:  modulePath,
			PackagePath: packagePath,
			ObjectName:  typeName + "." + method.Name(),
			Kind:        "method",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ObjectName < out[j].ObjectName
	})
	return out
}

func selectedInterfaceMethodSymbols(modulePath, packagePath, typeName string, iface *types.Interface, methods []string) []reportv2.SymbolIdentity {
	out := make([]reportv2.SymbolIdentity, 0, len(methods))
	for _, method := range methods {
		if !interfaceHasMethod(iface, method) {
			continue
		}
		out = append(out, reportv2.SymbolIdentity{
			ModulePath:  modulePath,
			PackagePath: packagePath,
			ObjectName:  typeName + "." + method,
			Kind:        "method",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ObjectName < out[j].ObjectName
	})
	return out
}

func displayModuleRoot(moduleRoot string) string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(moduleRoot)
	}
	if rel, err := filepath.Rel(wd, moduleRoot); err == nil && rel != "" && rel != "." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(moduleRoot)
}

func cloneMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func deriveAdapters(root reportv2.Root, shapeResult ShapeResult, classification *ArchetypeClassification) []reportv2.Adapter {
	var adapters []reportv2.Adapter
	for _, operation := range shapeResult.PerOperation {
		if operation.Shape == "http-handler" {
			adapterID := "net-http-handler"
			for _, evidence := range operation.Evidence {
				if strings.Contains(evidence, "caddyhttp.MiddlewareHandler") {
					adapterID = "caddy-middleware-handler"
					break
				}
			}
			adapters = append(adapters, reportv2.Adapter{
				Kind:                 "handler",
				ID:                   adapterID,
				MatchedSymbols:       []reportv2.SymbolIdentity{operation.Operation},
				CanonicalShapes:      []string{operation.Shape},
				StateEffects:         []string{},
				TransportEffects:     []string{"http"},
				SerializationEffects: []string{},
			})
			break
		}
	}
	if root.RegistryKey != nil {
		canonicalShapes := make([]string, 0, len(shapeResult.PerOperation))
		seen := map[string]bool{}
		for _, operation := range shapeResult.PerOperation {
			if operation.Shape == "" || seen[operation.Shape] {
				continue
			}
			seen[operation.Shape] = true
			canonicalShapes = append(canonicalShapes, operation.Shape)
		}
		sort.Strings(canonicalShapes)
		adapters = append(adapters, reportv2.Adapter{
			Kind:                 "registry",
			ID:                   "registry-keyed-root",
			MatchedSymbols:       []reportv2.SymbolIdentity{root.Identity},
			CanonicalShapes:      canonicalShapes,
			StateEffects:         []string{"immutable-config"},
			TransportEffects:     []string{},
			SerializationEffects: []string{},
		})
	}
	// The actor adapter is derived from the ADR-0022 classification primary,
	// not from legacy state classes or shape-only signals.
	if classification != nil && classification.Primary != nil && classification.Primary.Archetype == "serialized-actor" && classification.Primary.Emittable {
		adapters = append(adapters, reportv2.Adapter{
			Kind:                 "actor",
			ID:                   "serialized-actor",
			MatchedSymbols:       append([]reportv2.SymbolIdentity(nil), classification.MatchedSymbols...),
			CanonicalShapes:      append([]string(nil), classification.CanonicalShapes...),
			StateEffects:         []string{"serialized-owner", "mutex-serialized-state"},
			TransportEffects:     []string{"rpc-command-mailbox"},
			SerializationEffects: []string{"command-envelope"},
		})
	}
	return adapters
}

func applyArchetypeClassification(root *reportv2.Root, classification *ArchetypeClassification) {
	root.ArchetypeKind = classification.ArchetypeKind
	if classification.Primary != nil {
		primary := reportChoice(*classification.Primary, "")
		root.Primary = &primary
	}
	for _, alternative := range classification.Alternatives {
		root.Alternatives = append(root.Alternatives, reportChoice(alternative, "SUGGEST"))
	}
}

func reportChoice(choice ArchetypeChoice, verdict string) reportv2.ArchetypeChoice {
	return reportv2.ArchetypeChoice{
		Archetype:               choice.Archetype,
		ContributingArchetypes:  append([]string(nil), choice.ContributingArchetypes...),
		Alias:                   choice.Alias,
		Verdict:                 verdict,
		Emittable:               choice.Emittable,
		RuntimeSelectable:       choice.RuntimeSelectable,
		DynamicDelegateEligible: choice.DynamicDelegateEligible,
		RationaleTier:           choice.RationaleTier,
		Rationale:               truncateRationale(choice.Rationale),
	}
}

func truncateRationale(value string) string {
	if len(value) <= 140 {
		return value
	}
	return value[:140]
}
