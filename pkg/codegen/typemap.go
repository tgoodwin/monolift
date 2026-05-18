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
	// When a ResultDTO is applicable (> 1 non-error return), the caller
	// sets CodecResultDTO after building the DTO. Here we just classify
	// the first non-error result for backwards compatibility.
	result := results[0]
	return ReturnCodec{
		Kind:     result.Codec,
		Nullable: strings.HasPrefix(result.GoType, "*"),
		GoType:   result.GoType,
	}
}

// BuildResultDTO examines a plan's results and, if there are > 1 non-error
// returns whose types are all JSON-codable, builds a ResultDTO that packs
// them into a single struct. Returns nil if a DTO is not needed (0 or 1
// non-error result) or not possible (non-JSON-codable type in the results).
func BuildResultDTO(funcName string, results []Result) *ResultDTO {
	var nonError []Result
	for _, r := range results {
		if r.Codec == CodecError {
			continue
		}
		nonError = append(nonError, r)
	}
	if len(nonError) <= 1 {
		return nil
	}
	// Check all non-error results are JSON-codable.
	for _, r := range nonError {
		if !isJSONCodableResultType(r.GoType) {
			return nil
		}
	}
	name := "result"
	if funcName != "" {
		name = strings.ToLower(funcName[:1]) + funcName[1:] + "Result"
	}
	dto := &ResultDTO{
		Name: name,
	}
	for i, r := range nonError {
		fieldName := r.Name
		if fieldName == "" || fieldName == "result" {
			fieldName = "Result" + string(rune('0'+i))
		}
		dto.Fields = append(dto.Fields, ResultDTOField{
			Name:            exportedFieldName(fieldName),
			JSONName:        toSnake(fieldName),
			GoType:          r.GoType,
			QualifiedGoType: r.QualifiedGoType,
			Index:           r.Index,
			OriginalName:    r.Name,
		})
	}
	return dto
}

// isJSONCodableResultType returns true if the Go type string represents a
// type that can be packed into a JSON DTO. Channels, function types, sync
// primitives, and io.Reader/Writer types cannot.
func isJSONCodableResultType(goType string) bool {
	lower := strings.ToLower(goType)
	switch {
	case strings.Contains(lower, "chan "):
		return false
	case strings.Contains(lower, "io.reader"),
		strings.Contains(lower, "io.writer"),
		strings.Contains(lower, "io.readcloser"),
		strings.Contains(lower, "io.writecloser"),
		strings.Contains(lower, "io.readwriter"):
		return false
	case strings.Contains(lower, "sync."):
		return false
	case strings.HasPrefix(strings.TrimPrefix(goType, "*"), "func("),
		strings.HasPrefix(strings.TrimPrefix(goType, "*"), "func ("):
		return false
	case strings.Contains(lower, "http.responsewriter"):
		return false
	}
	return true
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
