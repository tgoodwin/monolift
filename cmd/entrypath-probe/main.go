package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/pkg/compiler/entrypath"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type stringFlags []string

func (s *stringFlags) String() string {
	return strings.Join(*s, ",")
}

func (s *stringFlags) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var rootSpecs stringFlags
	var diagnosticTimings bool
	var boundaryDiscoveryMode string
	var boundaryFrontierDepth int
	var boundaryFrontierMaxAdjacentOwners int
	var boundaryFrontierMaxBoundaryCandidates int
	var boundaryFrontierMaxDuration time.Duration
	var boundaryFrontierMaxOwners int
	var boundaryFrontierMaxPackages int
	var boundaryFrontierMaxReverseOwners int
	var bridgeMaxBoundaryOwners int
	var bridgeMaxDuration time.Duration
	var bridgeMaxInstructions int
	var bridgeMaxOwners int
	var bridgeMaxPackageFunctions int
	var bridgeMaxPackages int
	var bridgeMaxStarts int
	var functionIndexBudget time.Duration
	var functionIndexMaxFunctions int
	var functionIndexMode string
	var functionIndexProgressInterval int
	var oracleBridgeMaxDuration time.Duration
	var oracleBridgeMaxOwners int
	var oracleBridgeMaxPackageFunctions int
	var oracleSpecPath string
	var targetedMaxDepth int
	var targetedMaxDuration time.Duration
	var targetedMaxFunctions int
	var targetedMaxQueue int
	flag.Var(&rootSpecs, "region-root", "region root function or method, repeatable")
	flag.BoolVar(&diagnosticTimings, "diagnostic-timings", false, "write phase timing diagnostics to stderr")
	flag.StringVar(&boundaryDiscoveryMode, "boundary-discovery-mode", string(entrypath.BoundaryDiscoveryModeAll), "boundary predicate discovery mode: all or frontier")
	flag.IntVar(&boundaryFrontierMaxOwners, "boundary-frontier-max-owners", 0, "legacy maximum owners in boundary frontier discovery; 0 uses the diagnostic default")
	flag.IntVar(&boundaryFrontierMaxReverseOwners, "boundary-frontier-max-reverse-owners", 0, "maximum reverse owners in boundary frontier discovery; 0 uses the diagnostic default")
	flag.IntVar(&boundaryFrontierMaxAdjacentOwners, "boundary-frontier-max-adjacent-owners", 0, "maximum callgraph-adjacent owners in boundary frontier discovery; 0 uses the diagnostic default")
	flag.IntVar(&boundaryFrontierMaxBoundaryCandidates, "boundary-frontier-max-boundary-candidates", 0, "maximum owners scanned by boundary predicates in frontier discovery; 0 uses the diagnostic default")
	flag.IntVar(&boundaryFrontierDepth, "boundary-frontier-depth", 0, "maximum callgraph-adjacent expansion depth for boundary frontier discovery; 0 uses the diagnostic default")
	flag.IntVar(&boundaryFrontierMaxPackages, "boundary-frontier-max-packages", 0, "maximum packages admitted to boundary frontier discovery; 0 disables the package budget")
	flag.DurationVar(&boundaryFrontierMaxDuration, "boundary-frontier-max-duration", 0, "maximum elapsed duration for boundary frontier discovery; 0 uses the diagnostic default")
	flag.IntVar(&bridgeMaxStarts, "bridge-max-starts", 0, "maximum reverse-BFS touchpoint starts selected by bridge mode; 0 uses the diagnostic default")
	flag.IntVar(&bridgeMaxPackages, "bridge-max-packages", 0, "maximum packages scanned by bridge mode; 0 uses the diagnostic default")
	flag.IntVar(&bridgeMaxPackageFunctions, "bridge-max-package-functions", 0, "maximum same-package functions scanned per bridge package; 0 uses the diagnostic default")
	flag.IntVar(&bridgeMaxOwners, "bridge-max-owners", 0, "maximum bridge owner seeds; 0 uses the diagnostic default")
	flag.IntVar(&bridgeMaxBoundaryOwners, "bridge-max-boundary-owners", 0, "maximum owners scanned by bridge boundary predicates; 0 uses the diagnostic default")
	flag.IntVar(&bridgeMaxInstructions, "bridge-max-instructions", 0, "maximum bridge seed-discovery instructions scanned; 0 uses the diagnostic default")
	flag.DurationVar(&bridgeMaxDuration, "bridge-max-duration", 0, "maximum elapsed duration for bridge seed discovery; 0 uses the diagnostic default")
	flag.DurationVar(&functionIndexBudget, "function-index-budget", 0, "maximum duration for building the function reference index; 0 disables the internal budget")
	flag.IntVar(&functionIndexMaxFunctions, "function-index-max-functions", 0, "maximum number of sorted SSA functions to scan while building the function reference index; 0 scans all")
	flag.StringVar(&functionIndexMode, "function-index-mode", string(entrypath.FunctionIndexModeAll), "function reference index mode: all, reverse-path, http-sinks (legacy boundary seed spelling), targeted, bridge, or oracle-bridge")
	flag.IntVar(&functionIndexProgressInterval, "function-index-progress-interval", 0, "emit function reference index progress every n scanned instructions when --diagnostic-timings is enabled; 0 disables progress events")
	flag.StringVar(&oracleSpecPath, "oracle-spec", "", "path to oracle trace JSON spec")
	flag.IntVar(&oracleBridgeMaxPackageFunctions, "oracle-bridge-max-package-functions", 0, "maximum same-package functions scanned per oracle bridge start; 0 uses the diagnostic default")
	flag.IntVar(&oracleBridgeMaxOwners, "oracle-bridge-max-owners", 0, "maximum owner functions added by oracle bridge mode; 0 uses the diagnostic default")
	flag.DurationVar(&oracleBridgeMaxDuration, "oracle-bridge-max-duration", 0, "maximum elapsed duration for oracle bridge seed discovery; 0 uses the diagnostic default")
	flag.IntVar(&targetedMaxDepth, "targeted-max-depth", 1, "maximum targeted on-demand expansion depth")
	flag.DurationVar(&targetedMaxDuration, "targeted-max-duration", 30*time.Second, "maximum elapsed duration for targeted on-demand expansion")
	flag.IntVar(&targetedMaxFunctions, "targeted-max-functions", 10000, "maximum seeded functions scanned by targeted mode")
	flag.IntVar(&targetedMaxQueue, "targeted-max-queue", 100000, "maximum targeted expansion work items")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: entrypath-probe [--diagnostic-timings] [--boundary-discovery-mode=all|frontier] [--boundary-frontier-max-reverse-owners=<n>] [--boundary-frontier-max-adjacent-owners=<n>] [--boundary-frontier-max-boundary-candidates=<n>] [--boundary-frontier-depth=<n>] [--boundary-frontier-max-duration=<duration>] [--bridge-max-starts=<n>] [--bridge-max-packages=<n>] [--bridge-max-package-functions=<n>] [--bridge-max-owners=<n>] [--bridge-max-boundary-owners=<n>] [--bridge-max-instructions=<n>] [--bridge-max-duration=<duration>] [--function-index-budget=<duration>] [--function-index-max-functions=<n>] [--function-index-mode=all|reverse-path|http-sinks|targeted|bridge|oracle-bridge] [--function-index-progress-interval=<n>] [--oracle-spec=<path>] [--region-root=<function>] <package-dir-or-pattern>")
		os.Exit(2)
	}
	parsedBoundaryDiscoveryMode, err := entrypath.ParseBoundaryDiscoveryMode(boundaryDiscoveryMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "entrypath-probe: %v\n", err)
		os.Exit(2)
	}
	parsedFunctionIndexMode, err := entrypath.ParseFunctionIndexMode(functionIndexMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "entrypath-probe: %v\n", err)
		os.Exit(2)
	}
	if functionIndexBudget < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --function-index-budget must be non-negative")
		os.Exit(2)
	}
	if boundaryFrontierMaxOwners < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --boundary-frontier-max-owners must be non-negative")
		os.Exit(2)
	}
	if boundaryFrontierMaxReverseOwners < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --boundary-frontier-max-reverse-owners must be non-negative")
		os.Exit(2)
	}
	if boundaryFrontierMaxAdjacentOwners < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --boundary-frontier-max-adjacent-owners must be non-negative")
		os.Exit(2)
	}
	if boundaryFrontierMaxBoundaryCandidates < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --boundary-frontier-max-boundary-candidates must be non-negative")
		os.Exit(2)
	}
	if boundaryFrontierDepth < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --boundary-frontier-depth must be non-negative")
		os.Exit(2)
	}
	if boundaryFrontierMaxPackages < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --boundary-frontier-max-packages must be non-negative")
		os.Exit(2)
	}
	if boundaryFrontierMaxDuration < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --boundary-frontier-max-duration must be non-negative")
		os.Exit(2)
	}
	if bridgeMaxStarts < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --bridge-max-starts must be non-negative")
		os.Exit(2)
	}
	if bridgeMaxPackages < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --bridge-max-packages must be non-negative")
		os.Exit(2)
	}
	if bridgeMaxPackageFunctions < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --bridge-max-package-functions must be non-negative")
		os.Exit(2)
	}
	if bridgeMaxOwners < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --bridge-max-owners must be non-negative")
		os.Exit(2)
	}
	if bridgeMaxBoundaryOwners < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --bridge-max-boundary-owners must be non-negative")
		os.Exit(2)
	}
	if bridgeMaxInstructions < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --bridge-max-instructions must be non-negative")
		os.Exit(2)
	}
	if bridgeMaxDuration < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --bridge-max-duration must be non-negative")
		os.Exit(2)
	}
	if functionIndexMaxFunctions < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --function-index-max-functions must be non-negative")
		os.Exit(2)
	}
	if functionIndexProgressInterval < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --function-index-progress-interval must be non-negative")
		os.Exit(2)
	}
	if oracleBridgeMaxPackageFunctions < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --oracle-bridge-max-package-functions must be non-negative")
		os.Exit(2)
	}
	if oracleBridgeMaxOwners < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --oracle-bridge-max-owners must be non-negative")
		os.Exit(2)
	}
	if oracleBridgeMaxDuration < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --oracle-bridge-max-duration must be non-negative")
		os.Exit(2)
	}
	if targetedMaxDepth < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --targeted-max-depth must be non-negative")
		os.Exit(2)
	}
	if targetedMaxDuration < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --targeted-max-duration must be non-negative")
		os.Exit(2)
	}
	if targetedMaxFunctions < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --targeted-max-functions must be non-negative")
		os.Exit(2)
	}
	if targetedMaxQueue < 0 {
		fmt.Fprintln(os.Stderr, "entrypath-probe: --targeted-max-queue must be non-negative")
		os.Exit(2)
	}
	oracleSpec, err := readOracleSpec(oracleSpecPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "entrypath-probe: %v\n", err)
		os.Exit(2)
	}

	logger := newPhaseLogger(diagnosticTimings)
	prog, mainPkg, err := loadSSA(flag.Arg(0), logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "entrypath-probe: %v\n", err)
		os.Exit(1)
	}
	resolveStarted := logger.start("root_resolution")
	resolveRSSBefore := currentRSSBytes()
	roots, rootStats, rootDiagnostics, err := resolveRegionRoots(prog, rootSpecs)
	rootStats.ElapsedMillis = time.Since(resolveStarted.started).Milliseconds()
	rootStats.RSSDeltaBytes = int64(currentRSSBytes()) - int64(resolveRSSBefore)
	logger.end(resolveStarted)
	if err != nil {
		fmt.Fprintf(os.Stderr, "entrypath-probe: %v\n", err)
		os.Exit(1)
	}
	result, err := entrypath.ProbeWithOptions(prog, mainPkg, roots, entrypath.ProbeOptions{
		PhaseObserver:                         logger.observeEntryPath,
		FunctionIndexMode:                     parsedFunctionIndexMode,
		BoundaryDiscoveryMode:                 parsedBoundaryDiscoveryMode,
		BoundaryFrontierMaxOwners:             boundaryFrontierMaxOwners,
		BoundaryFrontierMaxReverseOwners:      boundaryFrontierMaxReverseOwners,
		BoundaryFrontierMaxAdjacentOwners:     boundaryFrontierMaxAdjacentOwners,
		BoundaryFrontierMaxBoundaryCandidates: boundaryFrontierMaxBoundaryCandidates,
		BoundaryFrontierDepth:                 boundaryFrontierDepth,
		BoundaryFrontierMaxPackages:           boundaryFrontierMaxPackages,
		BoundaryFrontierMaxDuration:           boundaryFrontierMaxDuration,
		FunctionRefIndexBudget:                functionIndexBudget,
		FunctionRefIndexMaxFunctions:          functionIndexMaxFunctions,
		FunctionRefIndexProgressInterval:      functionIndexProgressInterval,
		TargetedExpansionMaxDepth:             targetedMaxDepth,
		TargetedExpansionMaxDuration:          targetedMaxDuration,
		TargetedExpansionMaxFunctions:         targetedMaxFunctions,
		TargetedExpansionMaxQueue:             targetedMaxQueue,
		BridgeMaxStarts:                       bridgeMaxStarts,
		BridgeMaxPackages:                     bridgeMaxPackages,
		BridgeMaxPackageFunctions:             bridgeMaxPackageFunctions,
		BridgeMaxOwners:                       bridgeMaxOwners,
		BridgeMaxBoundaryOwners:               bridgeMaxBoundaryOwners,
		BridgeMaxInstructions:                 bridgeMaxInstructions,
		BridgeMaxDuration:                     bridgeMaxDuration,
		OracleSpec:                            oracleSpec,
		OracleBridgeMaxPackageFunctions:       oracleBridgeMaxPackageFunctions,
		OracleBridgeMaxOwners:                 oracleBridgeMaxOwners,
		OracleBridgeMaxDuration:               oracleBridgeMaxDuration,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "entrypath-probe: %v\n", err)
		os.Exit(1)
	}
	result.Stats.RootResolution = rootStats
	result.Diagnostics = append(rootDiagnostics, result.Diagnostics...)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "entrypath-probe: %v\n", err)
		os.Exit(1)
	}
	_, _ = os.Stdout.Write(append(data, '\n'))
}

func readOracleSpec(path string) (entrypath.OracleSpec, error) {
	if path == "" {
		return entrypath.OracleSpec{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return entrypath.OracleSpec{}, err
	}
	var spec entrypath.OracleSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return entrypath.OracleSpec{}, err
	}
	return spec, nil
}

func loadSSA(target string, logger phaseLogger) (*ssa.Program, *ssa.Package, error) {
	dir := ""
	pattern := target
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		abs, err := filepath.Abs(target)
		if err != nil {
			return nil, nil, err
		}
		dir = abs
		pattern = "."
	}
	cfg := &packages.Config{
		Mode:  packages.LoadAllSyntax | packages.NeedModule,
		Dir:   dir,
		Tests: false,
	}
	loadStarted := logger.start("package_load")
	pkgs, err := packages.Load(cfg, pattern)
	logger.end(loadStarted)
	if err != nil {
		return nil, nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, nil, fmt.Errorf("package load failed for %s", target)
	}
	ssaStarted := logger.start("ssa_build")
	prog, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	logger.end(ssaStarted)
	mainPkg := chooseMainPackage(ssaPkgs)
	if mainPkg == nil {
		return nil, nil, fmt.Errorf("main package not found for %s", target)
	}
	return prog, mainPkg, nil
}

func chooseMainPackage(pkgs []*ssa.Package) *ssa.Package {
	for _, pkg := range pkgs {
		if pkg != nil && pkg.Pkg != nil && pkg.Pkg.Name() == "main" {
			return pkg
		}
	}
	if len(pkgs) > 0 {
		return pkgs[0]
	}
	return nil
}

func resolveRegionRoots(prog *ssa.Program, specs []string) ([]*ssa.Function, entrypath.RootResolutionStats, []entrypath.Diagnostic, error) {
	var stats entrypath.RootResolutionStats
	var diagnostics []entrypath.Diagnostic
	var roots []*ssa.Function
	seen := map[*ssa.Function]bool{}
	var fallbackFunctions []*ssa.Function
	fallback := func() []*ssa.Function {
		if fallbackFunctions == nil {
			fallbackFunctions = sortedResolverFunctions(prog)
		}
		return fallbackFunctions
	}
	for _, spec := range specs {
		fn := findExactFunction(prog, spec, &stats)
		if fn != nil {
			stats.MatchedSpecs++
			stats.FastPathHits++
			if !seen[fn] {
				seen[fn] = true
				roots = append(roots, fn)
			}
			continue
		}
		fn, ambiguous := findFunction(fallback(), spec, &stats)
		if ambiguous {
			return nil, stats, diagnostics, fmt.Errorf("region root %q is ambiguous", spec)
		}
		if fn == nil {
			return nil, stats, diagnostics, fmt.Errorf("region root %q not found", spec)
		}
		stats.MatchedSpecs++
		stats.FallbackHits++
		diagnostics = append(diagnostics, entrypath.Diagnostic{
			Kind:     "root_resolution_fallback_used",
			Reason:   "root spec required suffix/fuzzy fallback resolution",
			Function: spec,
		})
		if !seen[fn] {
			seen[fn] = true
			roots = append(roots, fn)
		}
	}
	return roots, stats, diagnostics, nil
}

func sortedResolverFunctions(prog *ssa.Program) []*ssa.Function {
	functions := make([]*ssa.Function, 0, len(ssautil.AllFunctions(prog)))
	for fn := range ssautil.AllFunctions(prog) {
		if fn != nil {
			functions = append(functions, fn)
		}
	}
	sort.Slice(functions, func(i, j int) bool { return functions[i].String() < functions[j].String() })
	return functions
}

func findExactFunction(prog *ssa.Program, spec string, stats *entrypath.RootResolutionStats) *ssa.Function {
	if !isQualifiedRootSpec(spec) {
		return nil
	}
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil {
			continue
		}
		if stats != nil {
			stats.FunctionsInspected++
		}
		if functionMatchesExact(fn, spec) {
			return fn
		}
	}
	return nil
}

func findFunction(functions []*ssa.Function, spec string, stats *entrypath.RootResolutionStats) (*ssa.Function, bool) {
	var matches []*ssa.Function
	for _, fn := range functions {
		if stats != nil {
			stats.FunctionsInspected++
		}
		if functionMatches(fn, spec) {
			matches = append(matches, fn)
		}
	}
	if len(matches) > 1 {
		return nil, true
	}
	if len(matches) == 1 {
		return matches[0], false
	}
	return nil, false
}

func isQualifiedRootSpec(spec string) bool {
	pkgPath, objectName := splitQualifiedRootSpec(spec)
	return pkgPath != "" && objectName != ""
}

func splitQualifiedRootSpec(spec string) (string, string) {
	if strings.HasPrefix(spec, "(") {
		return "", ""
	}
	if idx := strings.LastIndex(spec, ".(*"); idx > 0 {
		return spec[:idx], spec[idx+1:]
	}
	if idx := strings.LastIndex(spec, ".("); idx > 0 {
		return spec[:idx], spec[idx+1:]
	}
	lastSlash := strings.LastIndex(spec, "/")
	lastDot := strings.LastIndex(spec, ".")
	if lastDot > lastSlash && lastDot > 0 && lastDot < len(spec)-1 {
		return spec[:lastDot], spec[lastDot+1:]
	}
	return "", ""
}

func functionMatchesExact(fn *ssa.Function, spec string) bool {
	objectName := functionObjectName(fn)
	packagePath := ""
	if fn.Package() != nil && fn.Package().Pkg != nil {
		packagePath = fn.Package().Pkg.Path()
	}
	return spec == packagePath+"."+objectName || fn.String() == spec
}

func functionMatches(fn *ssa.Function, spec string) bool {
	objectName := functionObjectName(fn)
	packagePath := ""
	if fn.Package() != nil && fn.Package().Pkg != nil {
		packagePath = fn.Package().Pkg.Path()
	}
	switch {
	case spec == objectName:
		return true
	case spec == packagePath+"."+objectName:
		return true
	case strings.HasSuffix(packagePath+"."+objectName, "."+spec):
		return true
	case fn.String() == spec:
		return true
	default:
		return false
	}
}

type phaseLogger struct {
	enabled bool
}

type phaseMark struct {
	name    string
	started time.Time
}

func newPhaseLogger(enabled bool) phaseLogger {
	return phaseLogger{enabled: enabled}
}

func (logger phaseLogger) start(name string) phaseMark {
	if logger.enabled {
		logger.write(name, "start", 0, currentRSSBytes())
	}
	return phaseMark{name: name, started: time.Now()}
}

func (logger phaseLogger) end(mark phaseMark) {
	if !logger.enabled || mark.name == "" {
		return
	}
	logger.write(mark.name, "end", time.Since(mark.started).Milliseconds(), currentRSSBytes())
}

func (logger phaseLogger) observeEntryPath(event entrypath.PhaseEvent) {
	if !logger.enabled {
		return
	}
	logger.writeEvent(event)
}

func (logger phaseLogger) write(name, status string, elapsedMillis int64, rssBytes uint64) {
	if !logger.enabled {
		return
	}
	fmt.Fprintf(os.Stderr, "entrypath-probe phase=%s status=%s elapsed_ms=%d rss_bytes=%d\n", name, status, elapsedMillis, rssBytes)
}

func (logger phaseLogger) writeEvent(event entrypath.PhaseEvent) {
	if !logger.enabled {
		return
	}
	fmt.Fprintf(os.Stderr, "entrypath-probe phase=%s status=%s elapsed_ms=%d rss_bytes=%d", event.Name, event.Status, event.WallClockMillis, event.PeakRSSBytes)
	if event.ScannedFunctions > 0 {
		fmt.Fprintf(os.Stderr, " scanned_functions=%d", event.ScannedFunctions)
	}
	if event.ScannedBlocks > 0 {
		fmt.Fprintf(os.Stderr, " scanned_blocks=%d", event.ScannedBlocks)
	}
	if event.ScannedInstructions > 0 {
		fmt.Fprintf(os.Stderr, " scanned_instructions=%d", event.ScannedInstructions)
	}
	if event.CurrentPackagePath != "" {
		fmt.Fprintf(os.Stderr, " current_package_path=%s", event.CurrentPackagePath)
	}
	fmt.Fprintln(os.Stderr)
}

func currentRSSBytes() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.Sys
}

func functionObjectName(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	if fn.Signature != nil && fn.Signature.Recv() != nil {
		return receiverName(fn.Signature.Recv().Type()) + "." + fn.Name()
	}
	return fn.Name()
}

func receiverName(typ types.Type) string {
	switch t := typ.(type) {
	case *types.Pointer:
		return "(*" + receiverName(t.Elem()) + ")"
	case *types.Named:
		return t.Obj().Name()
	default:
		return types.TypeString(typ, func(*types.Package) string { return "" })
	}
}
