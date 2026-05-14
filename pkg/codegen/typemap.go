package codegen

import (
	"go/types"
	"path"
	"strings"
	"unicode"

	"github.com/tgoodwin/monolift/pkg/activation"
)

func MapParam(funcName string, index int, param *types.Var, currentPkgPath string) (Param, error) {
	name := paramName(index, param)
	class, codec := classifyCodec(param.Type())
	return Param{
		Name:             name,
		JSONName:         jsonFieldName(funcName, name),
		GoType:           typeString(param.Type(), currentPkgPath),
		QualifiedGoType:  typeString(param.Type(), ""),
		TypePackagePath:  namedTypePackagePath(param.Type()),
		TypePackageAlias: packageAlias(namedTypePackagePath(param.Type())),
		Codec:            codec,
		Index:            index,
		Classification:   class,
	}, nil
}

func MapResult(index int, result *types.Var, currentPkgPath string) (Result, error) {
	name := result.Name()
	if name == "" {
		name = "result"
		if index > 0 {
			name = "result" + string(rune('0'+index))
		}
	}
	_, codec := classifyCodec(result.Type())
	return Result{
		Name:             name,
		JSONName:         jsonFieldName("", name),
		GoType:           typeString(result.Type(), currentPkgPath),
		QualifiedGoType:  typeString(result.Type(), ""),
		TypePackagePath:  namedTypePackagePath(result.Type()),
		TypePackageAlias: packageAlias(namedTypePackagePath(result.Type())),
		Codec:            codec,
		Index:            index,
	}, nil
}

func ReturnCodecFor(results []Result) ReturnCodec {
	if len(results) == 0 {
		return ReturnCodec{}
	}
	result := results[0]
	return ReturnCodec{
		Kind:     result.Codec,
		Nullable: strings.HasPrefix(result.GoType, "*"),
		GoType:   result.GoType,
	}
}

func classifyCodec(typ types.Type) (activation.BoundaryDataClass, Codec) {
	if typ == nil {
		return activation.BoundaryInfeasible, CodecJSON
	}
	if isErrorType(typ) {
		return activation.Serializable, CodecError
	}
	if isLocalizedErrorWrapper(typ) {
		return activation.Serializable, CodecLocalizedErrorWrapper
	}
	if isStreamingReader(typ) {
		return activation.Serializable, CodecStreamingBytes
	}
	switch t := types.Unalias(typ).(type) {
	case *types.Basic:
		info := t.Info()
		if info&types.IsString != 0 || info&types.IsBoolean != 0 || info&types.IsInteger != 0 || info&types.IsFloat != 0 {
			return activation.Trivial, CodecPrimitive
		}
	case *types.Pointer:
		class, _ := classifyCodec(t.Elem())
		if class == activation.Trivial {
			return activation.Serializable, CodecJSON
		}
		return activation.Serializable, CodecJSON
	case *types.Named:
		if basic, ok := types.Unalias(t.Underlying()).(*types.Basic); ok {
			info := basic.Info()
			if info&types.IsString != 0 || info&types.IsBoolean != 0 || info&types.IsInteger != 0 || info&types.IsFloat != 0 {
				return activation.Trivial, CodecPrimitive
			}
		}
	}
	return activation.Serializable, CodecJSON
}

func isErrorType(typ types.Type) bool {
	return types.Identical(typ, types.Universe.Lookup("error").Type())
}

func isStreamingReader(typ types.Type) bool {
	named := namedType(typ)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	if named.Obj().Pkg().Path() != "io" {
		return false
	}
	switch named.Obj().Name() {
	case "Reader", "ReadSeeker", "ReadCloser":
		return true
	}
	return false
}

func isLocalizedErrorWrapper(typ types.Type) bool {
	named := namedType(typ)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "miniflux.app/v2/internal/locale" && named.Obj().Name() == "LocalizedErrorWrapper"
}

func paramName(index int, param *types.Var) string {
	if param != nil && param.Name() != "" {
		return param.Name()
	}
	return "p" + string(rune('0'+index))
}

func typeString(typ types.Type, currentPkgPath string) string {
	return types.TypeString(typ, func(pkg *types.Package) string {
		if pkg == nil || pkg.Path() == currentPkgPath {
			return ""
		}
		return pkg.Name()
	})
}

func namedType(typ types.Type) *types.Named {
	typ = types.Unalias(typ)
	for {
		ptr, ok := typ.(*types.Pointer)
		if !ok {
			break
		}
		typ = types.Unalias(ptr.Elem())
	}
	named, _ := typ.(*types.Named)
	return named
}

func namedTypePackagePath(typ types.Type) string {
	named := namedType(typ)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return ""
	}
	return named.Obj().Pkg().Path()
}

func packageAlias(importPath string) string {
	if importPath == "" {
		return ""
	}
	return path.Base(importPath)
}

func jsonFieldName(funcName, name string) string {
	switch name {
	case "baseURL":
		return "base_url"
	case "rawHTML":
		if funcName == "SanitizeHTML" {
			return "input"
		}
		return "raw_html"
	case "userID":
		return "user_id"
	case "feedID":
		return "feed_id"
	case "forceRefresh":
		return "force_refresh"
	}
	return toSnake(name)
}

func toSnake(name string) string {
	var out []rune
	var prevLower bool
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 && prevLower {
				out = append(out, '_')
			}
			out = append(out, unicode.ToLower(r))
			prevLower = false
			continue
		}
		out = append(out, r)
		prevLower = unicode.IsLower(r) || unicode.IsDigit(r)
	}
	return string(out)
}
