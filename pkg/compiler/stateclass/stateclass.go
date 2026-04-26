package stateclass

import (
	"fmt"
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

type Class string

const (
	ClassStateless               Class = "stateless"
	ClassImmutableCapturedConfig Class = "immutable-captured-config"
	ClassProcessLocalCache       Class = "process-local-cache"
	ClassExternalizedDurable     Class = "externalized-durable"
	ClassSingletonMutable        Class = "singleton-mutable"
	ClassSharedMutableAcross     Class = "shared-mutable-across-callers"
	ClassConnectionSession       Class = "connection-session"
)

type Inference struct {
	Symbol            reportv2.SymbolIdentity
	Classes           []Class
	Disposition       string
	Evidence          []string
	DeveloperDeclared bool
}

type Result struct {
	Items             []Inference
	Diagnostics       []extract.Diagnostic
	PrecisionTriggers []string
}

var embeddedDBAppRootMethodThreshold = 100

func Infer(loaded *extract.LoadedModule, program *ssa.Program, reachable []*ssa.Function, root reportv2.Root, parsed *extract.Pragma) (Result, error) {
	if loaded == nil {
		return Result{}, fmt.Errorf("stateclass: loaded module is nil")
	}
	if program == nil {
		return Result{}, fmt.Errorf("stateclass: program is nil")
	}

	seeds := harvestSeeds(loaded, root, reachable)
	if len(seeds) == 0 {
		return inferNoSeedResult(root, parsed), nil
	}

	inferences := make([]Inference, 0, len(seeds))
	diagnostics := []extract.Diagnostic{}
	triggers := []string{}
	for _, seed := range seeds {
		inference, seedDiagnostics := inferSeed(loaded, root, seed, parsed)
		if inference.Symbol.ObjectName != "" {
			inferences = append(inferences, inference)
		}
		diagnostics = append(diagnostics, seedDiagnostics...)
	}
	if root.RegistryKey != nil {
		inferences, diagnostics = filterRegistryKeyedRows(inferences, diagnostics)
	}

	inferences, compositeDiagnostics, compositeTriggers := applyCompositeEmbeddedDBRule(loaded, root, seeds, inferences)
	diagnostics = append(diagnostics, compositeDiagnostics...)
	triggers = append(triggers, compositeTriggers...)

	items := coalesceInferences(root, inferences)
	sortDiagnostics(diagnostics)
	sort.Strings(triggers)
	triggers = compactStrings(triggers)
	return Result{
		Items:             items,
		Diagnostics:       diagnostics,
		PrecisionTriggers: triggers,
	}, nil
}

func filterRegistryKeyedRows(inferences []Inference, diagnostics []extract.Diagnostic) ([]Inference, []extract.Diagnostic) {
	filtered := make([]Inference, 0, len(inferences))
	for _, inference := range inferences {
		if len(inference.Classes) == 0 {
			continue
		}
		switch inference.Classes[0] {
		case ClassImmutableCapturedConfig, ClassExternalizedDurable:
			filtered = append(filtered, inference)
		}
	}
	filteredDiagnostics := diagnostics[:0]
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "MLV2_STATE_UNKNOWN" {
			continue
		}
		filteredDiagnostics = append(filteredDiagnostics, diagnostic)
	}
	return filtered, filteredDiagnostics
}

func ForExtract(loaded *extract.LoadedModule, program *ssa.Program, reachable []*ssa.Function, root reportv2.Root, parsed *extract.Pragma) (extract.StateResult, error) {
	result, err := Infer(loaded, program, reachable, root, parsed)
	if err != nil {
		return extract.StateResult{}, err
	}
	items := make([]reportv2.StateItem, 0, len(result.Items))
	for _, item := range result.Items {
		classes := make([]string, 0, len(item.Classes))
		for _, class := range item.Classes {
			classes = append(classes, string(class))
		}
		items = append(items, reportv2.StateItem{
			Symbol:            item.Symbol,
			Classes:           classes,
			Disposition:       item.Disposition,
			Evidence:          append([]string(nil), item.Evidence...),
			DeveloperDeclared: item.DeveloperDeclared,
		})
	}
	return extract.StateResult{
		Items:             items,
		Diagnostics:       append([]extract.Diagnostic(nil), result.Diagnostics...),
		PrecisionTriggers: append([]string(nil), result.PrecisionTriggers...),
		Classification:    classifyForExtract(loaded, root, reachable),
	}, nil
}

func classifyForExtract(loaded *extract.LoadedModule, root reportv2.Root, reachable []*ssa.Function) *extract.ArchetypeClassification {
	seeds := harvestSeeds(loaded, root, reachable)
	var selected *extract.ArchetypeClassification
	for _, seed := range seeds {
		classification := ClassifyRegion(regionEvidence(root.Properties, seed))
		if classification.Primary == nil {
			continue
		}
		converted := convertClassification(root, seed, classification)
		if selected == nil || converted.ArchetypeKind == "alternative_set" {
			selected = converted
		}
	}
	return selected
}

func convertClassification(root reportv2.Root, seed seed, classification Classification) *extract.ArchetypeClassification {
	out := &extract.ArchetypeClassification{
		ArchetypeKind:   classification.ArchetypeKind,
		RationaleTier:   string(classification.RationaleTier),
		RationaleProse:  classification.RationaleProse,
		MatchedSymbols:  []reportv2.SymbolIdentity{root.Identity, seed.identity},
		CanonicalShapes: []string{root.Shape},
	}
	if classification.Primary != nil {
		primary := convertCandidate(*classification.Primary, classification.RationaleTier, classification.RationaleProse)
		out.Primary = &primary
	}
	for _, alternative := range classification.Alternatives {
		choice := convertCandidate(alternative, classification.RationaleTier, "runtime selection is not hosted yet")
		out.Alternatives = append(out.Alternatives, choice)
	}
	return out
}

func convertCandidate(candidate Candidate, tier RationaleTier, prose string) extract.ArchetypeChoice {
	contributing := make([]string, 0, len(candidate.ContributingArchetypes))
	for _, archetype := range candidate.ContributingArchetypes {
		contributing = append(contributing, string(archetype))
	}
	return extract.ArchetypeChoice{
		Archetype:               string(candidate.Archetype),
		ContributingArchetypes:  contributing,
		Alias:                   candidate.Alias,
		Emittable:               Emittable(candidate),
		RuntimeSelectable:       RuntimeSelectable(candidate),
		DynamicDelegateEligible: DynamicDelegateEligible(candidate),
		RationaleTier:           string(tier),
		Rationale:               prose,
	}
}

type seed struct {
	identity       reportv2.SymbolIdentity
	typ            types.Type
	referenced     bool
	storeSites     []string
	syncWitnesses  []string
	channelLoop    bool
	keyedAccess    bool
	mutexProtected bool
}

func inferNoSeedResult(root reportv2.Root, parsed *extract.Pragma) Result {
	if parsed == nil || parsed.Options["state"] == "" {
		return Result{}
	}
	class, disposition := declaredStateClass(parsed.Options["state"])
	return Result{
		Items: []Inference{{
			Symbol:            root.Identity,
			Classes:           []Class{class},
			Disposition:       disposition,
			Evidence:          []string{"no captured mutable state was observed"},
			DeveloperDeclared: true,
		}},
	}
}

func inferSeed(loaded *extract.LoadedModule, root reportv2.Root, seed seed, parsed *extract.Pragma) (Inference, []extract.Diagnostic) {
	class, evidence := inferClass(seed)
	developerDeclared := false
	diagnostics := []extract.Diagnostic{}
	disposition := dispositionForClass(class)

	declared := ""
	if parsed != nil {
		declared = parsed.Options["state"]
	}

	switch {
	case declared == "stateless" && (class == ClassSharedMutableAcross || class == ClassSingletonMutable || class == ClassConnectionSession):
		diagnostics = append(diagnostics, diagnostic(loaded.RootPragma.Span, "MLV2_STATE_DECL_CONFLICT", "declared state=stateless conflicts with mutation evidence", "SS-CLASS-3"))
		disposition = "refused"
	case class == "" && declared != "":
		class, disposition = declaredStateClass(declared)
		developerDeclared = true
		evidence = []string{"developer declaration selected the state class"}
	case class == "" && declared == "":
		diagnostics = append(diagnostics, diagnostic(loaded.RootPragma.Span, "MLV2_STATE_UNKNOWN", "state class remains correctness-relevant and ambiguous", "SS-CLASS-4"))
		class = ClassStateless
		disposition = "refused"
	case declared != "" && declared != "stateless":
		developerDeclared = true
		_, disposition = declaredStateClass(declared)
	}

	if len(evidence) == 0 {
		evidence = []string{"state classification matched no stronger evidence"}
	}
	sort.Strings(evidence)
	return Inference{
		Symbol:            seed.identity,
		Classes:           []Class{class},
		Disposition:       disposition,
		Evidence:          evidence,
		DeveloperDeclared: developerDeclared,
	}, diagnostics
}

func regionEvidence(rootProperties []reportv2.PropertyEvidence, seed seed) []liftability.Evidence {
	out := evidenceFromReportProperties(rootProperties)
	if seed.identity.Kind == "field" {
		out = append(out, liftability.Evidence{
			PropertyID: liftability.PropertyStateReceiverOwnedState,
			Subject:    seed.identity.ObjectName,
			Verdict:    liftability.VerdictHold,
			Source:     liftability.SourceSSA,
			Detail:     "receiver field owned by root type",
		})
	}
	if seed.mutexProtected {
		out = append(out, liftability.Evidence{
			PropertyID: liftability.PropertyStateMutexEnclosesStoreInvariant,
			Subject:    seed.identity.ObjectName,
			Verdict:    liftability.VerdictHold,
			Source:     liftability.SourceSSA,
			Detail:     "store occurs in function with mutex lock/unlock witness",
		})
	}
	if seed.keyedAccess {
		out = append(out, liftability.Evidence{
			PropertyID: liftability.PropertyStateKeyedAccessInvariant,
			Subject:    seed.identity.ObjectName,
			Verdict:    liftability.VerdictHold,
			Source:     liftability.SourceSSA,
			Detail:     "map region is updated by key",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PropertyID != out[j].PropertyID {
			return out[i].PropertyID < out[j].PropertyID
		}
		return out[i].Subject < out[j].Subject
	})
	return out
}

func evidenceFromReportProperties(properties []reportv2.PropertyEvidence) []liftability.Evidence {
	out := make([]liftability.Evidence, 0, len(properties))
	for _, property := range properties {
		out = append(out, liftability.Evidence{
			PropertyID: liftability.PropertyID(property.PropertyID),
			Subject:    property.Subject,
			Verdict:    liftability.Verdict(property.Verdict),
			Source:     liftability.Source(property.Source),
			Detail:     property.Detail,
		})
	}
	return out
}

// site:begin state-class-rules
func inferClass(seed seed) (Class, []string) {
	if class, evidence, ok := externalClientTypeRule(seed.typ); ok {
		return class, evidence
	}
	if class, evidence, ok := sharedGlobalMutationRule(seed); ok {
		return class, evidence
	}
	if class, evidence, ok := syncPrimitiveRule(seed); ok {
		return class, evidence
	}
	if class, evidence, ok := channelLoopRule(seed); ok {
		return class, evidence
	}
	if class, evidence, ok := mutationFreeReadRule(seed); ok {
		return class, evidence
	}
	if class, evidence, ok := stackLocalRule(seed); ok {
		return class, evidence
	}
	return "", nil
}

// site:end state-class-rules

func sharedGlobalMutationRule(seed seed) (Class, []string, bool) {
	if seed.identity.Kind == "variable" && len(seed.storeSites) > 0 {
		return ClassSharedMutableAcross, []string{fmt.Sprintf("package global mutated at %s", seed.storeSites[0])}, true
	}
	return "", nil, false
}

func syncPrimitiveRule(seed seed) (Class, []string, bool) {
	if len(seed.syncWitnesses) > 0 {
		return ClassSharedMutableAcross, []string{fmt.Sprintf("sync witness %s", seed.syncWitnesses[0])}, true
	}
	return "", nil, false
}

func channelLoopRule(seed seed) (Class, []string, bool) {
	if seed.channelLoop {
		return ClassSingletonMutable, []string{"mutation occurs inside a channel-driven loop"}, true
	}
	return "", nil, false
}

func mutationFreeReadRule(seed seed) (Class, []string, bool) {
	if len(seed.storeSites) == 0 {
		return ClassImmutableCapturedConfig, []string{"captured state is read-only in the reachable closure"}, true
	}
	return "", nil, false
}

func stackLocalRule(seed seed) (Class, []string, bool) {
	if seed.identity.Kind == "freevar" && len(seed.storeSites) == 0 {
		return ClassStateless, []string{"captured stack-local value does not escape shared state"}, true
	}
	return "", nil, false
}

func applyCompositeEmbeddedDBRule(loaded *extract.LoadedModule, root reportv2.Root, seeds []seed, inferences []Inference) ([]Inference, []extract.Diagnostic, []string) {
	if len(root.ExposedOperations) <= embeddedDBAppRootMethodThreshold {
		return inferences, nil, nil
	}
	matching := map[string]seed{}
	for _, seed := range seeds {
		if seed.identity.Kind != "field" || !isEmbeddedDBClientType(seed.typ) {
			continue
		}
		matching[seed.identity.ObjectName] = seed
	}
	if len(matching) == 0 {
		return inferences, nil, nil
	}

	var filtered []Inference
	for _, inference := range inferences {
		seed, ok := matching[inference.Symbol.ObjectName]
		if !ok {
			continue
		}
		inference.Classes = []Class{ClassExternalizedDurable}
		inference.Disposition = "refused"
		inference.DeveloperDeclared = false
		inference.Evidence = []string{fmt.Sprintf("%s field on embedded-DB app root", typeLabel(seed.typ))}
		filtered = append(filtered, inference)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Symbol.ObjectName < filtered[j].Symbol.ObjectName
	})

	diagnostics := []extract.Diagnostic{
		diagnostic(loaded.RootPragma.Span, "MLV2_EMBEDDED_DB_APP_ROOT", "embedded database app root selected as lift root", "SS-LIFT-6", "SS-DISP-2"),
		diagnostic(loaded.RootPragma.Span, "MLV2_CLOSURE_TOO_LARGE", "root closure exceeds the bounded precision threshold", "EC-PRUNE-3"),
	}
	return filtered, diagnostics, []string{"embedded-db", "closure-size"}
}

func coalesceInferences(root reportv2.Root, inferences []Inference) []Inference {
	if len(inferences) == 0 {
		return nil
	}

	groups := map[string][]Inference{}
	order := []string{}
	for _, inference := range inferences {
		key := inference.Disposition + "|" + strings.Join(classStrings(inference.Classes), ",") + "|" + strings.Join(inference.Evidence, ",") + "|" + boolString(inference.DeveloperDeclared)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], inference)
	}

	out := make([]Inference, 0, len(groups))
	for _, key := range order {
		group := groups[key]
		if len(group) == 0 {
			continue
		}
		if group[0].Disposition == "refused" {
			out = append(out, group...)
			continue
		}
		merged := group[0]
		if len(group) > 1 || group[0].Classes[0] == ClassImmutableCapturedConfig {
			merged.Symbol = root.Identity
		}
		out = append(out, merged)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Symbol.ObjectName < out[j].Symbol.ObjectName
	})
	return out
}

func harvestSeeds(loaded *extract.LoadedModule, root reportv2.Root, reachable []*ssa.Function) []seed {
	seeds := map[string]*seed{}
	prepopulateReceiverFields(loaded, seeds, root, reachable)
	for _, fn := range reachable {
		if fn == nil {
			continue
		}
		syncWitnesses := collectSyncWitnesses(loaded, fn)
		channelLoop := functionHasChannelLoop(fn)
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				switch typed := instr.(type) {
				case *ssa.Store:
					if global, ok := typed.Addr.(*ssa.Global); ok {
						if !samePackageGlobal(root, global) {
							break
						}
						entry := ensureSeed(seeds, globalIdentity(root, global), derefType(global.Type()))
						entry.referenced = true
						entry.storeSites = append(entry.storeSites, sourceSite(loaded, typed.Pos()))
						entry.syncWitnesses = append(entry.syncWitnesses, syncWitnesses...)
						entry.channelLoop = entry.channelLoop || channelLoop
					}
					if fieldSeed, ok := fieldSeedFromAddr(root, typed.Addr); ok {
						entry := ensureSeed(seeds, fieldSeed.identity, fieldSeed.typ)
						entry.referenced = true
						entry.storeSites = append(entry.storeSites, sourceSite(loaded, typed.Pos()))
						entry.syncWitnesses = append(entry.syncWitnesses, syncWitnesses...)
						entry.channelLoop = entry.channelLoop || channelLoop
					}
				case *ssa.MapUpdate:
					if global, ok := typed.Map.(*ssa.Global); ok {
						if !samePackageGlobal(root, global) {
							break
						}
						entry := ensureSeed(seeds, globalIdentity(root, global), derefType(global.Type()))
						entry.referenced = true
						entry.storeSites = append(entry.storeSites, sourceSite(loaded, typed.Pos()))
						entry.syncWitnesses = append(entry.syncWitnesses, syncWitnesses...)
						entry.channelLoop = entry.channelLoop || channelLoop
					}
					if fieldSeed, ok := fieldSeedFromMapValue(root, typed.Map); ok {
						entry := ensureSeed(seeds, fieldSeed.identity, fieldSeed.typ)
						entry.referenced = true
						entry.keyedAccess = true
						entry.storeSites = append(entry.storeSites, sourceSite(loaded, typed.Pos()))
						entry.syncWitnesses = append(entry.syncWitnesses, syncWitnesses...)
						entry.mutexProtected = entry.mutexProtected || len(syncWitnesses) > 0
					}
				}
				if fieldSeed, ok := fieldSeedFromInstruction(root, instr); ok {
					entry := ensureSeed(seeds, fieldSeed.identity, fieldSeed.typ)
					entry.referenced = true
				}
				for _, operand := range instr.Operands(nil) {
					if operand == nil || *operand == nil {
						continue
					}
					if global, ok := (*operand).(*ssa.Global); ok {
						if !samePackageGlobal(root, global) {
							continue
						}
						entry := ensureSeed(seeds, globalIdentity(root, global), derefType(global.Type()))
						entry.referenced = true
					}
				}
			}
		}
	}

	ownersWithSync := ownersWithSyncFields(loaded)
	for _, seed := range seeds {
		if seed == nil || seed.identity.Kind != "field" {
			continue
		}
		if isSyncPrimitiveType(seed.typ) || isMutexNamedField(seed.identity.ObjectName) {
			ownersWithSync[fieldOwner(seed.identity.ObjectName)] = true
		}
	}

	var out []seed
	for _, seed := range seeds {
		if seed == nil || seed.identity.ObjectName == "" {
			continue
		}
		if skipSeed(*seed) {
			continue
		}
		seed.storeSites = compactStrings(seed.storeSites)
		seed.syncWitnesses = compactStrings(seed.syncWitnesses)
		seed.keyedAccess = seed.keyedAccess || isMapType(seed.typ)
		seed.mutexProtected = seed.mutexProtected ||
			len(seed.syncWitnesses) > 0 ||
			(len(seed.storeSites) > 0 && ownersWithSync[fieldOwner(seed.identity.ObjectName)]) ||
			(len(seed.storeSites) > 0 && !isMapType(seed.typ)) ||
			strings.Contains(strings.ToLower(seed.identity.ObjectName), "connections")
		out = append(out, *seed)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].identity.ObjectName < out[j].identity.ObjectName
	})
	return out
}

func prepopulateReceiverFields(loaded *extract.LoadedModule, seeds map[string]*seed, root reportv2.Root, reachable []*ssa.Function) {
	owners := map[*types.Named]bool{}
	if root.Identity.Kind == "interface" {
		ifaceObj := findNamedInRootPackage(loaded, root.Identity.ObjectName)
		if ifaceObj != nil {
			iface, ok := ifaceObj.Underlying().(*types.Interface)
			if ok {
				iface.Complete()
				for owner := range implementingNamedTypes(loaded, iface, root.Identity.PackagePath) {
					owners[owner] = true
				}
			}
		}
	}
	for _, fn := range reachable {
		if fn == nil || fn.Signature == nil || fn.Signature.Recv() == nil {
			continue
		}
		owner := namedOwner(fn.Signature.Recv().Type())
		if owner == nil || owner.Obj() == nil || owner.Obj().Pkg() == nil {
			continue
		}
		if owner.Obj().Pkg().Path() != root.Identity.PackagePath {
			continue
		}
		if root.Identity.Kind == "type" && owner.Obj().Name() != root.Identity.ObjectName {
			continue
		}
		owners[owner] = true
	}
	for owner := range owners {
		strct, ok := owner.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < strct.NumFields(); i++ {
			field := strct.Field(i)
			if field == nil {
				continue
			}
			identity := reportv2.SymbolIdentity{
				ModulePath:  root.Identity.ModulePath,
				PackagePath: owner.Obj().Pkg().Path(),
				ObjectName:  owner.Obj().Name() + "." + field.Name(),
				Kind:        "field",
			}
			ensureSeed(seeds, identity, field.Type())
		}
	}
}

func skipSeed(seed seed) bool {
	if seed.identity.Kind == "variable" {
		if len(seed.storeSites) == 0 {
			_, _, ok := externalClientTypeRule(seed.typ)
			return !ok
		}
		return false
	}
	if _, ok := derefType(seed.typ).(*types.Chan); ok && len(seed.storeSites) == 0 {
		return true
	}
	if !seed.referenced && len(seed.storeSites) == 0 {
		_, _, ok := externalClientTypeRule(seed.typ)
		return !ok
	}
	return false
}

func ensureSeed(seeds map[string]*seed, identity reportv2.SymbolIdentity, typ types.Type) *seed {
	if existing := seeds[identity.ObjectName]; existing != nil {
		return existing
	}
	entry := &seed{identity: identity, typ: typ}
	seeds[identity.ObjectName] = entry
	return entry
}

func globalIdentity(root reportv2.Root, global *ssa.Global) reportv2.SymbolIdentity {
	pkgPath := root.Identity.PackagePath
	if global != nil && global.Package() != nil && global.Package().Pkg != nil {
		pkgPath = global.Package().Pkg.Path()
	}
	name := "<global>"
	if global != nil {
		name = global.Name()
	}
	return reportv2.SymbolIdentity{
		ModulePath:  root.Identity.ModulePath,
		PackagePath: pkgPath,
		ObjectName:  pkgPath + "." + name,
		Kind:        "variable",
	}
}

func samePackageGlobal(root reportv2.Root, global *ssa.Global) bool {
	return global != nil && global.Package() != nil && global.Package().Pkg != nil && global.Package().Pkg.Path() == root.Identity.PackagePath
}

func fieldSeedFromInstruction(root reportv2.Root, instr ssa.Instruction) (seed, bool) {
	switch typed := instr.(type) {
	case *ssa.Field:
		return fieldSeedFromRef(root, typed.X.Type(), typed.Field)
	case *ssa.FieldAddr:
		return fieldSeedFromRef(root, typed.X.Type(), typed.Field)
	default:
		return seed{}, false
	}
}

func fieldSeedFromAddr(root reportv2.Root, addr ssa.Value) (seed, bool) {
	fieldAddr, ok := addr.(*ssa.FieldAddr)
	if !ok {
		return seed{}, false
	}
	return fieldSeedFromRef(root, fieldAddr.X.Type(), fieldAddr.Field)
}

func fieldSeedFromMapValue(root reportv2.Root, value ssa.Value) (seed, bool) {
	switch typed := value.(type) {
	case *ssa.Field:
		return fieldSeedFromRef(root, typed.X.Type(), typed.Field)
	case *ssa.FieldAddr:
		return fieldSeedFromRef(root, typed.X.Type(), typed.Field)
	default:
		return seed{}, false
	}
}

func fieldSeedFromRef(root reportv2.Root, ownerType types.Type, fieldIndex int) (seed, bool) {
	owner := namedOwner(ownerType)
	if owner == nil {
		return seed{}, false
	}
	if owner.Obj() == nil || owner.Obj().Pkg() == nil {
		return seed{}, false
	}
	if owner.Obj().Pkg().Path() != root.Identity.PackagePath {
		return seed{}, false
	}
	if root.Identity.Kind == "type" && owner.Obj().Name() != root.Identity.ObjectName {
		return seed{}, false
	}
	strct, ok := owner.Underlying().(*types.Struct)
	if !ok || fieldIndex < 0 || fieldIndex >= strct.NumFields() {
		return seed{}, false
	}
	field := strct.Field(fieldIndex)
	if field == nil {
		return seed{}, false
	}
	pkgPath := root.Identity.PackagePath
	if owner.Obj() != nil && owner.Obj().Pkg() != nil {
		pkgPath = owner.Obj().Pkg().Path()
	}
	return seed{
		identity: reportv2.SymbolIdentity{
			ModulePath:  root.Identity.ModulePath,
			PackagePath: pkgPath,
			ObjectName:  owner.Obj().Name() + "." + field.Name(),
			Kind:        "field",
		},
		typ: field.Type(),
	}, true
}

func findNamedInRootPackage(loaded *extract.LoadedModule, name string) *types.Named {
	if loaded == nil || loaded.RootPkg == nil || loaded.RootPkg.Types == nil {
		return nil
	}
	if obj := loaded.RootPkg.Types.Scope().Lookup(name); obj != nil {
		named, _ := obj.Type().(*types.Named)
		return named
	}
	return nil
}

func implementingNamedTypes(loaded *extract.LoadedModule, iface *types.Interface, packagePath string) map[*types.Named]bool {
	owners := map[*types.Named]bool{}
	if loaded == nil || loaded.RootPkg == nil || loaded.RootPkg.Types == nil {
		return owners
	}
	scope := loaded.RootPkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		owner, ok := typeName.Type().(*types.Named)
		if !ok || owner.Obj() == nil || owner.Obj().Pkg() == nil || owner.Obj().Pkg().Path() != packagePath {
			continue
		}
		if types.Implements(types.NewPointer(owner), iface) || types.Implements(owner, iface) {
			owners[owner] = true
		}
	}
	return owners
}

func namedOwner(typ types.Type) *types.Named {
	switch typed := typ.(type) {
	case *types.Pointer:
		return namedOwner(typed.Elem())
	case *types.Named:
		return typed
	default:
		return nil
	}
}

func collectSyncWitnesses(loaded *extract.LoadedModule, fn *ssa.Function) []string {
	var witnesses []string
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			if syncCallPackage(call.Common()) {
				witnesses = append(witnesses, call.Common().String()+" @ "+sourceSite(loaded, instr.Pos()))
			}
		}
	}
	return compactStrings(witnesses)
}

func syncCallPackage(common *ssa.CallCommon) bool {
	if common == nil {
		return false
	}
	if callee := common.StaticCallee(); callee != nil && callee.Package() != nil && callee.Package().Pkg != nil {
		pkgPath := callee.Package().Pkg.Path()
		return pkgPath == "sync" || pkgPath == "sync/atomic"
	}
	if common.Method != nil && common.Method.Pkg() != nil {
		pkgPath := common.Method.Pkg().Path()
		return pkgPath == "sync" || pkgPath == "sync/atomic"
	}
	return false
}

func functionHasChannelLoop(fn *ssa.Function) bool {
	hasReceive := false
	hasBackEdge := false
	for _, block := range fn.Blocks {
		for _, succ := range block.Succs {
			if succ != nil && succ.Index <= block.Index {
				hasBackEdge = true
			}
		}
		for _, instr := range block.Instrs {
			switch typed := instr.(type) {
			case *ssa.UnOp:
				if typed.Op == token.ARROW {
					hasReceive = true
				}
			case *ssa.Select:
				for _, state := range typed.States {
					if state.Dir == types.RecvOnly || state.Dir == types.SendRecv {
						hasReceive = true
					}
				}
			}
		}
	}
	return hasReceive && hasBackEdge
}

func externalClientTypeRule(typ types.Type) (Class, []string, bool) {
	base := derefType(typ)
	if named := namedType(base); named != nil && named.Obj() != nil && named.Obj().Pkg() != nil {
		pkgPath := named.Obj().Pkg().Path()
		name := named.Obj().Name()
		switch {
		case pkgPath == "database/sql" && (name == "DB" || name == "Tx" || name == "Stmt"):
			return ClassExternalizedDurable, []string{fmt.Sprintf("external client type %s.%s", pkgPath, name)}, true
		case pkgPath == "github.com/pocketbase/dbx":
			return ClassExternalizedDurable, []string{fmt.Sprintf("external client type %s.%s", pkgPath, name)}, true
		case pkgPath == "net/http" && name == "Client":
			return ClassExternalizedDurable, []string{"external client type net/http.Client"}, true
		case pkgPath == "sync" && name == "Map":
			return ClassSharedMutableAcross, []string{"shared sync.Map state crosses callers"}, true
		}
	}
	if isContextType(base) {
		return "", nil, false
	}
	return "", nil, false
}

func dispositionForClass(class Class) string {
	switch class {
	case ClassStateless, ClassImmutableCapturedConfig:
		return "replicated"
	case ClassProcessLocalCache, ClassSingletonMutable:
		return "singleton"
	case ClassExternalizedDurable:
		return "externalize-required"
	case ClassConnectionSession:
		return "affinity-routed"
	case ClassSharedMutableAcross:
		return "refused"
	default:
		return "refused"
	}
}

func declaredStateClass(value string) (Class, string) {
	switch value {
	case "singleton":
		return ClassSingletonMutable, "singleton"
	case "affinity":
		return ClassConnectionSession, "affinity-routed"
	case "external":
		return ClassExternalizedDurable, "externalize-required"
	default:
		return ClassStateless, "replicated"
	}
}

func isEmbeddedDBClientType(typ types.Type) bool {
	base := derefType(typ)
	named := namedType(base)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	pkgPath := named.Obj().Pkg().Path()
	name := named.Obj().Name()
	return (pkgPath == "github.com/pocketbase/dbx") ||
		(pkgPath == "database/sql" && name == "DB")
}

func typeLabel(typ types.Type) string {
	base := derefType(typ)
	named := namedType(base)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return "stateful"
	}
	return named.Obj().Name()
}

func namedType(typ types.Type) *types.Named {
	named, _ := typ.(*types.Named)
	return named
}

func derefType(typ types.Type) types.Type {
	if ptr, ok := typ.(*types.Pointer); ok {
		return derefType(ptr.Elem())
	}
	return typ
}

func isMapType(typ types.Type) bool {
	_, ok := derefType(typ).Underlying().(*types.Map)
	return ok
}

func isSyncPrimitiveType(typ types.Type) bool {
	base := derefType(typ)
	named := namedType(base)
	return named != nil && named.Obj() != nil && named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == "sync" &&
		(named.Obj().Name() == "Mutex" || named.Obj().Name() == "RWMutex")
}

func ownersWithSyncFields(loaded *extract.LoadedModule) map[string]bool {
	out := map[string]bool{}
	if loaded == nil || loaded.RootPkg == nil || loaded.RootPkg.Types == nil {
		return out
	}
	for _, name := range loaded.RootPkg.Types.Scope().Names() {
		obj := loaded.RootPkg.Types.Scope().Lookup(name)
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		owner, ok := typeName.Type().(*types.Named)
		if !ok || owner.Obj() == nil || owner.Obj().Name() == "" {
			continue
		}
		strct, ok := owner.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < strct.NumFields(); i++ {
			field := strct.Field(i)
			if field != nil && (isSyncPrimitiveType(field.Type()) || isMutexNamedField(owner.Obj().Name()+"."+field.Name())) {
				out[owner.Obj().Name()] = true
			}
		}
	}
	return out
}

func fieldOwner(objectName string) string {
	if before, _, ok := strings.Cut(objectName, "."); ok {
		return before
	}
	return ""
}

func isMutexNamedField(objectName string) bool {
	_, field, ok := strings.Cut(objectName, ".")
	if !ok {
		return false
	}
	lower := strings.ToLower(field)
	return strings.Contains(lower, "mutex") || strings.HasSuffix(lower, "mu")
}

func isContextType(typ types.Type) bool {
	named := namedType(typ)
	return named != nil && named.Obj() != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
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
		return diags[i].Message < diags[j].Message
	})
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	var prev string
	for i, value := range values {
		if i == 0 || value != prev {
			out = append(out, value)
			prev = value
		}
	}
	return out
}

func classStrings(classes []Class) []string {
	out := make([]string, 0, len(classes))
	for _, class := range classes {
		out = append(out, string(class))
	}
	return out
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func sourceSite(loaded *extract.LoadedModule, pos token.Pos) string {
	position := loaded.Fset.Position(pos)
	if position.Filename == "" {
		return "unknown"
	}
	relative, err := filepath.Rel(loaded.ModuleRoot, position.Filename)
	if err != nil {
		relative = position.Filename
	}
	return filepath.ToSlash(relative) + ":" + fmt.Sprintf("%d", position.Line)
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
