package codegen

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/tgoodwin/monolift/pkg/activation"
)

// receiverFactoryEntry describes a known factory function for a receiver type.
type receiverFactoryEntry struct {
	FactoryFunc     string
	ConstructorArgs []string
}

// receiverFactoryRegistry maps "pkgPath.TypeName" to a factory entry.
// These are types whose unexported fields or non-trivial construction
// requires a known factory rather than JSON deserialization.
var receiverFactoryRegistry = map[string]receiverFactoryEntry{
	"code.gitea.io/gitea/modules/auth/password/hash.Argon2Hasher": {
		FactoryFunc:     "NewArgon2Hasher",
		ConstructorArgs: []string{`""`},
	},
	"github.com/mattermost/mattermost/server/v8/channels/app/password/hashers.PBKDF2": {
		FactoryFunc: "DefaultPBKDF2",
	},
	"example.com/receivermod/receivertest.FactoryBuilt": {
		FactoryFunc: "NewFactoryBuilt",
	},
}

// LookupReceiverFactory checks the factory registry for a known constructor.
func LookupReceiverFactory(named *types.Named) (string, bool) {
	entry, ok := lookupReceiverFactoryEntry(named)
	if !ok {
		return "", false
	}
	return entry.FactoryFunc, true
}

func lookupReceiverFactoryEntry(named *types.Named) (receiverFactoryEntry, bool) {
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return receiverFactoryEntry{}, false
	}
	key := named.Obj().Pkg().Path() + "." + named.Obj().Name()
	entry, ok := receiverFactoryRegistry[key]
	if !ok {
		return receiverFactoryEntry{}, false
	}
	return entry, true
}

// selectReceiverPolicy determines the receiver strategy for a method cut point.
// It returns a ReceiverSpec or an error if the receiver cannot be handled.
func selectReceiverPolicy(named *types.Named, isPointer bool, stateClass activation.StateClass) (*ReceiverSpec, error) {
	goType := named.Obj().Name()
	if isPointer {
		goType = "*" + goType
	}

	// 1. Check factory registry first.
	if factory, ok := lookupReceiverFactoryEntry(named); ok {
		return &ReceiverSpec{
			GoType:      goType,
			IsPointer:   isPointer,
			Policy:      ReceiverFactory,
			FactoryFunc: factory.FactoryFunc,
			FactoryArgs: append([]string(nil), factory.ConstructorArgs...),
		}, nil
	}

	receiverType := types.Type(named)
	if isPointer {
		receiverType = types.NewPointer(named)
	}
	if reconstructor, ok := LookupReconstructor(receiverType); ok {
		return &ReceiverSpec{
			GoType:        goType,
			IsPointer:     isPointer,
			Policy:        ReceiverReconstructed,
			Reconstructor: reconstructor,
		}, nil
	}

	// For boundary and zero policies, require Stateless or ConfigOnly state class.
	// ConfigOnly receivers hold immutable configuration data that is safe to serialize.
	if stateClass != activation.Stateless && stateClass != activation.ConfigOnly {
		return nil, fmt.Errorf("receiver_requires_reconstruction: receiver %s has state class %s", goType, stateClass)
	}

	strct, ok := types.Unalias(named.Underlying()).(*types.Struct)
	if !ok {
		return nil, fmt.Errorf("receiver_requires_reconstruction: receiver %s underlying type is not a struct", goType)
	}

	// 2. If all fields are exported and JSON-serializable → boundary.
	if isReceiverSerializable(strct) {
		return &ReceiverSpec{
			GoType:    goType,
			IsPointer: isPointer,
			Policy:    ReceiverBoundary,
			Codec:     CodecJSON,
		}, nil
	}

	// 3. If all fields are zero-safe → zero.
	if isReceiverZeroSafe(strct) {
		return &ReceiverSpec{
			GoType:    goType,
			IsPointer: isPointer,
			Policy:    ReceiverZero,
		}, nil
	}

	return nil, fmt.Errorf("receiver_requires_reconstruction: receiver %s has non-serializable fields", goType)
}

// isReceiverSerializable reports whether all fields in the struct are exported
// and have JSON-serializable types (no channels, funcs, sync primitives, or
// io.Reader/Writer).
func isReceiverSerializable(strct *types.Struct) bool {
	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		if !field.Exported() {
			return false
		}
		if !isFieldTypeSerializable(field.Type()) {
			return false
		}
	}
	return true
}

// isReceiverZeroSafe reports whether all fields are basic types with useful
// zero values, or the struct is empty (method namespace).
func isReceiverZeroSafe(strct *types.Struct) bool {
	if strct.NumFields() == 0 {
		return true
	}
	for i := 0; i < strct.NumFields(); i++ {
		if !isZeroSafeType(strct.Field(i).Type()) {
			return false
		}
	}
	return true
}

// isFieldTypeSerializable checks whether a type can be JSON-serialized.
func isFieldTypeSerializable(typ types.Type) bool {
	switch t := types.Unalias(typ).(type) {
	case *types.Basic:
		return true
	case *types.Pointer:
		return isFieldTypeSerializable(t.Elem())
	case *types.Slice:
		return isFieldTypeSerializable(t.Elem())
	case *types.Array:
		return isFieldTypeSerializable(t.Elem())
	case *types.Map:
		return isFieldTypeSerializable(t.Key()) && isFieldTypeSerializable(t.Elem())
	case *types.Named:
		// Check for known non-serializable types.
		if t.Obj() != nil && t.Obj().Pkg() != nil {
			pkgPath := t.Obj().Pkg().Path()
			name := t.Obj().Name()
			switch {
			case pkgPath == "database/sql" && name == "DB":
				return false
			case pkgPath == "net/http" && name == "Client":
				return false
			case strings.HasPrefix(pkgPath, "sync"):
				return false
			case pkgPath == "io" && (name == "Reader" || name == "Writer" || name == "ReadCloser" || name == "ReadSeeker"):
				return false
			}
		}
		return isFieldTypeSerializable(t.Underlying())
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if !t.Field(i).Exported() {
				return false
			}
			if !isFieldTypeSerializable(t.Field(i).Type()) {
				return false
			}
		}
		return true
	case *types.Interface:
		// The built-in error interface is JSON-serializable (marshals as string or null).
		if t.NumMethods() == 1 {
			m := t.Method(0)
			if m.Name() == "Error" {
				sig, ok := m.Type().(*types.Signature)
				if ok && sig.Params().Len() == 0 && sig.Results().Len() == 1 {
					if basic, ok := sig.Results().At(0).Type().(*types.Basic); ok && basic.Kind() == types.String {
						return true
					}
				}
			}
		}
		return false
	case *types.Chan:
		return false
	case *types.Signature:
		return false
	default:
		return false
	}
}

// isZeroSafeType checks whether a type has a meaningful zero value.
func isZeroSafeType(typ types.Type) bool {
	switch t := types.Unalias(typ).(type) {
	case *types.Basic:
		info := t.Info()
		return info&types.IsString != 0 || info&types.IsBoolean != 0 ||
			info&types.IsInteger != 0 || info&types.IsFloat != 0
	case *types.Named:
		return isZeroSafeType(t.Underlying())
	default:
		return false
	}
}

// lookupReceiverType finds the named type for a receiver in a loaded package.
func lookupReceiverType(pkg *types.Package, receiver string) (*types.Named, bool, error) {
	isPointer := strings.HasPrefix(receiver, "*")
	typeName := strings.TrimPrefix(receiver, "*")

	obj := pkg.Scope().Lookup(typeName)
	if obj == nil {
		return nil, false, fmt.Errorf("codegen: receiver type %s not found in %s", typeName, pkg.Path())
	}
	tn, ok := obj.(*types.TypeName)
	if !ok {
		return nil, false, fmt.Errorf("codegen: %s is not a type in %s", typeName, pkg.Path())
	}
	named, ok := tn.Type().(*types.Named)
	if !ok {
		return nil, false, fmt.Errorf("codegen: %s is not a named type in %s", typeName, pkg.Path())
	}
	return named, isPointer, nil
}

// lookupMethod finds a method on a named type's method set.
func lookupMethod(named *types.Named, isPointer bool, methodName string) (*types.Func, *types.Signature, error) {
	// For pointer receivers, use the pointer method set; for value receivers, use the named type directly.
	var mset *types.MethodSet
	if isPointer {
		mset = types.NewMethodSet(types.NewPointer(named))
	} else {
		mset = types.NewMethodSet(named)
	}
	for i := 0; i < mset.Len(); i++ {
		sel := mset.At(i)
		if sel.Obj().Name() == methodName {
			fn, ok := sel.Obj().(*types.Func)
			if !ok {
				continue
			}
			sig, ok := fn.Type().(*types.Signature)
			if !ok {
				continue
			}
			return fn, sig, nil
		}
	}
	return nil, nil, fmt.Errorf("codegen: method %s not found on %s", methodName, named.Obj().Name())
}
