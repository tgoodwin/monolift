package codegen

import (
	"go/types"
	"strings"
)

func LookupReconstructor(typ types.Type) (Reconstructor, bool) {
	if typ == nil {
		return Reconstructor{}, false
	}
	if recon, ok := directReconstructor(typ); ok {
		return recon, true
	}
	if recon, ok := sqlWrapperReconstructor(typ); ok {
		return recon, true
	}
	return Reconstructor{}, false
}

func directReconstructor(typ types.Type) (Reconstructor, bool) {
	named := namedType(typ)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return Reconstructor{}, false
	}
	pkgPath := named.Obj().Pkg().Path()
	typeName := named.Obj().Name()
	pointer := strings.HasPrefix(typeString(typ, ""), "*")
	switch {
	case pointer && pkgPath == "database/sql" && typeName == "DB":
		return Reconstructor{
			ID:      "sql_db",
			Type:    typeString(typ, ""),
			Imports: []string{"database/sql", "os", "_ github.com/lib/pq"},
		}, true
	case pointer && pkgPath == "net/http" && typeName == "Client":
		return Reconstructor{
			ID:      "http_client",
			Type:    typeString(typ, ""),
			Imports: []string{"net/http", "time"},
		}, true
	case pointer && pkgPath == "log" && typeName == "Logger":
		return Reconstructor{
			ID:      "logger",
			Type:    typeString(typ, ""),
			Imports: []string{"log", "os"},
		}, true
	default:
		return Reconstructor{}, false
	}
}

func sqlWrapperReconstructor(typ types.Type) (Reconstructor, bool) {
	named := namedType(typ)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return Reconstructor{}, false
	}
	strct, ok := types.Unalias(named.Underlying()).(*types.Struct)
	if !ok {
		return Reconstructor{}, false
	}
	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		fieldNamed := namedType(field.Type())
		if fieldNamed == nil || fieldNamed.Obj() == nil || fieldNamed.Obj().Pkg() == nil {
			continue
		}
		if fieldNamed.Obj().Pkg().Path() == "database/sql" && fieldNamed.Obj().Name() == "DB" {
			pkgPath := named.Obj().Pkg().Path()
			typeName := named.Obj().Name()
			return Reconstructor{
				ID:                      "sql_db_wrapper",
				Type:                    typeString(typ, ""),
				Imports:                 []string{"database/sql", "os", "_ github.com/lib/pq", pkgPath},
				ConstructorPackagePath:  pkgPath,
				ConstructorPackageAlias: packageAlias(pkgPath),
				ConstructorName:         "New" + typeName,
				CloseSource:             "db.Close()",
			}, true
		}
	}
	return Reconstructor{}, false
}
