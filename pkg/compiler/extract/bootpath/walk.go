package bootpath

import (
	"go/constant"
	"path"
	"sort"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/surface"
	"golang.org/x/tools/go/ssa"
)

func Walk(prog *ssa.Program, mainPkg *ssa.Package, regionName string, regionSurface surface.RegionSurface, union []*ssa.Function) (BootSpec, error) {
	_ = prog
	_ = regionName
	_ = regionSurface
	unionSet := map[*ssa.Function]bool{}
	unionNames := map[string]bool{}
	unionShortNames := map[string]bool{}
	for _, fn := range union {
		if fn != nil {
			unionSet[fn] = true
			unionNames[fn.String()] = true
			unionShortNames[fn.Name()] = true
		}
	}
	functions := append([]*ssa.Function(nil), union...)
	if mainPkg != nil {
		if member, ok := mainPkg.Members["main"].(*ssa.Function); ok {
			functions = append(functions, member)
		}
	}
	functions = uniqueSortedFunctions(functions)
	spec := BootSpec{}
	sourceSeen := map[string]bool{}
	depSeen := map[string]bool{}
	goSeen := map[string]bool{}
	refusalSeen := map[string]bool{}
	for _, fn := range functions {
		if fn == nil {
			continue
		}
		spec.EntryPath = append(spec.EntryPath, fn.String())
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				switch typed := instr.(type) {
				case *ssa.Go:
					if callee := typed.Common().StaticCallee(); callee != nil && (unionSet[callee] || unionNames[callee.String()]) {
						key := callee.String()
						if !goSeen[key] {
							goSeen[key] = true
							spec.GoroutineLaunches = append(spec.GoroutineLaunches, GoroutineLaunch{Callee: key, SSAOrigin: typed.Pos()})
						}
					} else if fn, ok := typed.Common().Value.(*ssa.Function); ok && unionNames[fn.String()] {
						key := fn.String()
						if !goSeen[key] {
							goSeen[key] = true
							spec.GoroutineLaunches = append(spec.GoroutineLaunches, GoroutineLaunch{Callee: key, SSAOrigin: typed.Pos()})
						}
					} else {
						callText := typed.Common().String()
						for short := range unionShortNames {
							if strings.Contains(callText, short) && !goSeen[short] {
								goSeen[short] = true
								spec.GoroutineLaunches = append(spec.GoroutineLaunches, GoroutineLaunch{Callee: short, SSAOrigin: typed.Pos()})
								break
							}
						}
					}
				case ssa.CallInstruction:
					recordCall(&spec, sourceSeen, depSeen, refusalSeen, typed)
				}
				for _, operand := range instr.Operands(nil) {
					if operand == nil || *operand == nil {
						continue
					}
					if c, ok := (*operand).(*ssa.Const); ok && c.Value != nil && c.Value.Kind() == constant.String {
						value := constant.StringVal(c.Value)
						key := "literal:" + value
						if !sourceSeen[key] {
							sourceSeen[key] = true
							spec.ConfigSources = append(spec.ConfigSources, LiteralSource{Value: value, SSAOrigin: c.Pos()})
						}
						if isHostOnlyPath(value) {
							refusalKey := RefusalUnportableLiteralPath + ":" + value
							if !refusalSeen[refusalKey] {
								refusalSeen[refusalKey] = true
								spec.Refusals = append(spec.Refusals, BootPathRefusal{Kind: RefusalUnportableLiteralPath, Message: value, SSAOrigin: c.Pos()})
							}
						}
					}
				}
			}
		}
	}
	sortBootSpec(&spec)
	return spec, nil
}

func recordCall(spec *BootSpec, sourceSeen, depSeen, refusalSeen map[string]bool, call ssa.CallInstruction) {
	common := call.Common()
	if common == nil {
		return
	}
	callee := common.StaticCallee()
	if callee == nil {
		return
	}
	name := callee.String()
	switch {
	case name == "os.Getenv" || name == "os.LookupEnv":
		if value, ok := constStringArg(common, 0); ok {
			key := "env:" + value
			if !sourceSeen[key] {
				sourceSeen[key] = true
				spec.ConfigSources = append(spec.ConfigSources, EnvSource{Name: value, Required: name == "os.LookupEnv", SSAOrigin: call.Pos()})
			}
		}
	case name == "flag.String":
		if value, ok := constStringArg(common, 0); ok {
			key := "flag:" + value
			if !sourceSeen[key] {
				sourceSeen[key] = true
				def, _ := constStringArg(common, 1)
				spec.ConfigSources = append(spec.ConfigSources, FlagSource{Name: value, Default: def, Required: false, FlagSet: "flag.CommandLine", SSAOrigin: call.Pos()})
			}
		}
	case name == "flag.Var":
		if value, ok := constStringArg(common, 1); ok {
			key := "flag:" + value
			if !sourceSeen[key] {
				sourceSeen[key] = true
				spec.ConfigSources = append(spec.ConfigSources, FlagSource{Name: value, Required: false, FlagSet: "flag.CommandLine", SSAOrigin: call.Pos()})
			}
		}
	case name == "os.Open" || name == "os.ReadFile":
		if value, ok := constStringArg(common, 0); ok {
			key := "file:" + value
			if !sourceSeen[key] {
				sourceSeen[key] = true
				spec.ConfigSources = append(spec.ConfigSources, FileSource{Path: value, Format: formatForPath(value), MountName: mountName(value), Required: true, SSAOrigin: call.Pos()})
			}
		}
	case strings.Contains(name, "database/sql.(*DB).QueryRow"):
		key := "db:" + name
		if !sourceSeen[key] {
			sourceSeen[key] = true
			spec.ConfigSources = append(spec.ConfigSources, DBSource{Name: name, QueryShape: "QueryRow", Required: true, SSAOrigin: call.Pos()})
		}
	default:
		if dep := classifyDependency(name); dep != "" {
			if !depSeen[name] {
				depSeen[name] = true
				spec.DependencyInits = append(spec.DependencyInits, DependencyInit{Name: name, Classification: dep, SSAOrigin: call.Pos()})
			}
		}
	}
}

func constStringArg(common *ssa.CallCommon, index int) (string, bool) {
	if common == nil || index >= len(common.Args) {
		return "", false
	}
	c, ok := common.Args[index].(*ssa.Const)
	if !ok || c.Value == nil || c.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(c.Value), true
}

func classifyDependency(name string) string {
	switch {
	case strings.Contains(name, "platform.NewService"):
		return DependencySubstitutable
	case strings.Contains(name, "app.New"), strings.Contains(name, "server.New"), strings.Contains(name, "HubsStart"):
		return DependencyRequired
	case strings.Contains(strings.ToLower(name), "plugin"):
		return DependencyDisabledByMinimalConfig
	}
	return ""
}

func uniqueSortedFunctions(functions []*ssa.Function) []*ssa.Function {
	seen := map[*ssa.Function]bool{}
	var out []*ssa.Function
	for _, fn := range functions {
		if fn != nil && !seen[fn] {
			seen[fn] = true
			out = append(out, fn)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func sortBootSpec(spec *BootSpec) {
	sort.Slice(spec.ConfigSources, func(i, j int) bool {
		left := spec.ConfigSources[i].Kind() + ":" + spec.ConfigSources[i].Identifier()
		right := spec.ConfigSources[j].Kind() + ":" + spec.ConfigSources[j].Identifier()
		return left < right
	})
	sort.Slice(spec.DependencyInits, func(i, j int) bool { return spec.DependencyInits[i].Name < spec.DependencyInits[j].Name })
	sort.Slice(spec.GoroutineLaunches, func(i, j int) bool { return spec.GoroutineLaunches[i].Callee < spec.GoroutineLaunches[j].Callee })
	sort.Slice(spec.Refusals, func(i, j int) bool {
		return spec.Refusals[i].Kind+spec.Refusals[i].Message < spec.Refusals[j].Kind+spec.Refusals[j].Message
	})
	sort.Strings(spec.EntryPath)
}

func formatForPath(value string) FileFormat {
	if strings.HasSuffix(strings.ToLower(value), ".json") {
		return FileFormatJSON
	}
	return FileFormatUnknown
}

func mountName(value string) string {
	base := path.Base(value)
	base = strings.TrimSuffix(base, path.Ext(base))
	base = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return '-'
	}, base)
	base = strings.Trim(base, "-")
	if base == "" {
		return "config"
	}
	return base
}

func isHostOnlyPath(value string) bool {
	return strings.HasPrefix(value, "/etc/host-only") || strings.Contains(value, "host-only-state")
}
