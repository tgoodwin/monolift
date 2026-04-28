package stateclass

import (
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/ssa"
)

type SeamType string

const (
	SeamChannelField SeamType = "ChannelField"
	SeamMutexField   SeamType = "MutexField"
	SeamAtomicField  SeamType = "AtomicField"
)

type Seam struct {
	Type     SeamType
	Field    string
	ElemType string
	Writers  []string
	Readers  []string
	Span     reportv2.SourceSpan
	Evidence string
}

func DetectSeams(reachableByRoot map[string][]*ssa.Function) []Seam {
	accesses := map[string]*seamAccess{}
	rootSet := map[string]bool{}
	for rootID := range reachableByRoot {
		rootSet[rootID] = true
	}
	for rootID, funcs := range reachableByRoot {
		for _, fn := range funcs {
			if fn == nil {
				continue
			}
			for _, block := range fn.Blocks {
				for _, instr := range block.Instrs {
					switch typed := instr.(type) {
					case *ssa.Send:
						if field := fieldAccess(typed.Chan); field != nil && field.kind == SeamChannelField {
							ensureSeamAccess(accesses, field).addWriter(rootID, typed.Pos(), fn, field)
						}
					case *ssa.UnOp:
						if typed.Op == token.ARROW {
							if field := fieldAccess(typed.X); field != nil && field.kind == SeamChannelField {
								ensureSeamAccess(accesses, field).addReader(rootID, typed.Pos(), fn, field)
							}
						}
					case *ssa.Select:
						for _, state := range typed.States {
							if field := fieldAccess(state.Chan); field != nil && field.kind == SeamChannelField {
								if state.Dir == types.SendOnly {
									ensureSeamAccess(accesses, field).addWriter(rootID, state.Pos, fn, field)
								} else if state.Dir == types.RecvOnly {
									ensureSeamAccess(accesses, field).addReader(rootID, state.Pos, fn, field)
								}
							}
						}
					case ssa.CallInstruction:
						for _, value := range callValues(typed) {
							if field := fieldAccess(value); field != nil && (field.kind == SeamMutexField || field.kind == SeamAtomicField) {
								access := ensureSeamAccess(accesses, field)
								access.addReader(rootID, typed.Pos(), fn, field)
								access.addWriter(rootID, typed.Pos(), fn, field)
							}
						}
					}
				}
			}
		}
	}

	out := make([]Seam, 0, len(accesses))
	for _, access := range accesses {
		access.writers = sortedSet(access.writerSet)
		access.readers = sortedSet(access.readerSet)
		switch access.kind {
		case SeamChannelField:
			if hasRootOwnedChannelField(accesses, rootSet) && !rootSet[access.owner] {
				continue
			}
			if sameStringSet(access.writerSet, access.readerSet) {
				continue
			}
			if len(access.writers) == 0 || len(access.readers) == 0 {
				continue
			}
		case SeamMutexField, SeamAtomicField:
			if len(unionStringSets(access.writerSet, access.readerSet)) < 2 {
				continue
			}
		default:
			continue
		}
		out = append(out, access.toSeam())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].Field != out[j].Field {
			return out[i].Field < out[j].Field
		}
		if strings.Join(out[i].Writers, "\x00") != strings.Join(out[j].Writers, "\x00") {
			return strings.Join(out[i].Writers, "\x00") < strings.Join(out[j].Writers, "\x00")
		}
		return strings.Join(out[i].Readers, "\x00") < strings.Join(out[j].Readers, "\x00")
	})
	return out
}

func SeamsToReport(seams []Seam) []reportv2.SeamEntry {
	out := make([]reportv2.SeamEntry, 0, len(seams))
	for _, seam := range seams {
		out = append(out, reportv2.SeamEntry{
			Type:     string(seam.Type),
			Field:    seam.Field,
			ElemType: seam.ElemType,
			Writers:  append([]string(nil), seam.Writers...),
			Readers:  append([]string(nil), seam.Readers...),
			Span:     seam.Span,
			Evidence: seam.Evidence,
		})
	}
	return out
}

func ForExtractSeams(_ *extract.LoadedModule, _ *ssa.Program, reachableByRoot map[string][]*ssa.Function) ([]reportv2.SeamEntry, error) {
	return SeamsToReport(DetectSeams(reachableByRoot)), nil
}

type seamField struct {
	kind     SeamType
	owner    string
	name     string
	elemType string
}

func (f seamField) key() string {
	return string(f.kind) + "|" + f.owner + "." + f.name
}

type seamAccess struct {
	kind      SeamType
	owner     string
	field     string
	elemType  string
	span      reportv2.SourceSpan
	evidence  string
	writerSet map[string]bool
	readerSet map[string]bool
	writers   []string
	readers   []string
}

func ensureSeamAccess(accesses map[string]*seamAccess, field *seamField) *seamAccess {
	key := field.key()
	access := accesses[key]
	if access == nil {
		access = &seamAccess{}
		accesses[key] = access
	}
	return access
}

func (a *seamAccess) addWriter(rootID string, pos token.Pos, fn *ssa.Function, field *seamField) {
	a.init(field, pos, fn)
	a.writerSet[rootID] = true
}

func (a *seamAccess) addReader(rootID string, pos token.Pos, fn *ssa.Function, field *seamField) {
	a.init(field, pos, fn)
	a.readerSet[rootID] = true
}

func (a *seamAccess) init(field *seamField, pos token.Pos, fn *ssa.Function) {
	if a.writerSet == nil {
		a.writerSet = map[string]bool{}
	}
	if a.readerSet == nil {
		a.readerSet = map[string]bool{}
	}
	if a.kind == "" {
		a.kind = field.kind
		a.owner = field.owner
		a.field = field.owner + "." + field.name
		a.elemType = field.elemType
		a.evidence = evidenceForFunction(fn)
		if fn != nil && fn.Prog != nil {
			a.span = spanForPos(fn.Prog.Fset, pos)
		}
	}
}

func hasRootOwnedChannelField(accesses map[string]*seamAccess, rootSet map[string]bool) bool {
	for _, access := range accesses {
		if access.kind == SeamChannelField && rootSet[access.owner] {
			return true
		}
	}
	return false
}

func (a *seamAccess) toSeam() Seam {
	return Seam{
		Type:     a.kind,
		Field:    a.field,
		ElemType: a.elemType,
		Writers:  append([]string(nil), a.writers...),
		Readers:  append([]string(nil), a.readers...),
		Span:     a.span,
		Evidence: a.evidence,
	}
}

func fieldAccess(value ssa.Value) *seamField {
	for value != nil {
		switch typed := value.(type) {
		case *ssa.FieldAddr:
			return seamFieldFromSelection(typed.X.Type(), typed.Field)
		case *ssa.Field:
			return seamFieldFromSelection(typed.X.Type(), typed.Field)
		case *ssa.UnOp:
			value = typed.X
		case *ssa.ChangeType:
			value = typed.X
		case *ssa.Convert:
			value = typed.X
		default:
			return nil
		}
	}
	return nil
}

func seamFieldFromSelection(ownerType types.Type, fieldIndex int) *seamField {
	named, strct := namedStruct(ownerType)
	if named == nil || strct == nil || fieldIndex < 0 || fieldIndex >= strct.NumFields() {
		return nil
	}
	field := strct.Field(fieldIndex)
	if field == nil {
		return nil
	}
	typ := derefType(field.Type())
	switch typed := typ.(type) {
	case *types.Chan:
		return &seamField{
			kind:     SeamChannelField,
			owner:    named.Obj().Name(),
			name:     field.Name(),
			elemType: types.TypeString(typed.Elem(), packageQualifier),
		}
	case *types.Named:
		obj := typed.Obj()
		if obj == nil || obj.Pkg() == nil {
			return nil
		}
		path := obj.Pkg().Path()
		if path == "sync" && (obj.Name() == "Mutex" || obj.Name() == "RWMutex") {
			return &seamField{kind: SeamMutexField, owner: named.Obj().Name(), name: field.Name(), elemType: types.TypeString(field.Type(), packageQualifier)}
		}
		if path == "sync/atomic" {
			return &seamField{kind: SeamAtomicField, owner: named.Obj().Name(), name: field.Name(), elemType: types.TypeString(field.Type(), packageQualifier)}
		}
	}
	return nil
}

func namedStruct(typ types.Type) (*types.Named, *types.Struct) {
	typ = derefType(typ)
	named, ok := typ.(*types.Named)
	if !ok {
		return nil, nil
	}
	strct, _ := named.Underlying().(*types.Struct)
	return named, strct
}

func callValues(call ssa.CallInstruction) []ssa.Value {
	common := call.Common()
	out := make([]ssa.Value, 0, 1+len(common.Args))
	if common.Value != nil {
		out = append(out, common.Value)
	}
	out = append(out, common.Args...)
	return out
}

func evidenceForFunction(fn *ssa.Function) string {
	if fn == nil {
		return "ssa field access"
	}
	return "ssa field access in " + seamFunctionObjectName(fn)
}

func seamFunctionObjectName(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	if recv := fn.Signature.Recv(); recv != nil {
		return seamReceiverName(recv.Type()) + "." + fn.Name()
	}
	return fn.Name()
}

func seamReceiverName(typ types.Type) string {
	switch typed := typ.(type) {
	case *types.Pointer:
		return "(*" + seamReceiverName(typed.Elem()) + ")"
	case *types.Named:
		return typed.Obj().Name()
	default:
		return types.TypeString(typ, packageQualifier)
	}
}

func spanForPos(fset *token.FileSet, pos token.Pos) reportv2.SourceSpan {
	position := fset.Position(pos)
	return reportv2.SourceSpan{
		FileRelativePath: position.Filename,
		ByteOffsetStart:  position.Offset,
		ByteOffsetEnd:    position.Offset,
		LineStart:        position.Line,
		LineEnd:          position.Line,
	}
}

func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sameStringSet(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if !right[value] {
			return false
		}
	}
	return true
}

func unionStringSets(left, right map[string]bool) map[string]bool {
	out := map[string]bool{}
	for value := range left {
		out[value] = true
	}
	for value := range right {
		out[value] = true
	}
	return out
}

func packageQualifier(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	return pkg.Path()
}
