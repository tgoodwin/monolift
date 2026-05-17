package codegen

import (
	"go/types"
	"strings"
)

func LookupReconstructor(typ types.Type) (Reconstructor, bool) {
	if typ == nil {
		return Reconstructor{}, false
	}
	if recon, ok := contextReconstructor(typ); ok {
		return recon, true
	}
	if recon, ok := loggerInterfaceReconstructor(typ); ok {
		return recon, true
	}
	if recon, ok := directReconstructor(typ); ok {
		return recon, true
	}
	if recon, ok := sqlWrapperReconstructor(typ); ok {
		return recon, true
	}
	return Reconstructor{}, false
}

func hasKnownReceiverReconstructor(pkgPath, typeName string) bool {
	_, ok := receiverReconstructorID(pkgPath, typeName)
	return ok
}

func receiverReconstructorID(pkgPath, typeName string) (string, bool) {
	switch pkgPath + "." + typeName {
	case "github.com/pocketbase/pocketbase/tools/filesystem.System":
		return "pocketbase_local_filesystem", true
	default:
		return "", false
	}
}

type sqlWrapperEntry struct {
	ConstructorFunc     string
	ConstructorArgOrder []string
}

var sqlWrapperRegistry = map[string]sqlWrapperEntry{
	"miniflux.app/v2/internal/storage.Storage": {
		ConstructorFunc:     "NewStorage",
		ConstructorArgOrder: []string{"db"},
	},
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
			ID:          "sql_db",
			Type:        typeString(typ, ""),
			Imports:     []string{"context", "database/sql", "os", "_ github.com/lib/pq"},
			CloseSource: "db.Close()",
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
	case pointer && pkgPath == "github.com/pocketbase/pocketbase/tools/filesystem" && typeName == "System":
		return filesystemSystemReconstructor(typ), true
	default:
		return Reconstructor{}, false
	}
}

func filesystemSystemReconstructor(typ types.Type) Reconstructor {
	return Reconstructor{
		ID:      "pocketbase_local_filesystem",
		Type:    typeString(typ, ""),
		Imports: []string{"fmt", "os", "path/filepath", "github.com/pocketbase/pocketbase/tools/filesystem"},
		InitLines: []string{
			`$ROOT_VAR := os.Getenv("MONOLIFT_FILESYSTEM_ROOT")`,
			`if $ROOT_VAR == "" { return nil, fmt.Errorf("MONOLIFT_FILESYSTEM_ROOT is required") }`,
			`$CLEAN_ROOT_VAR, err := filepath.Abs($ROOT_VAR)`,
			`if err != nil { return nil, err }`,
		},
		StartupProbeLines: []string{
			`if err := os.MkdirAll($CLEAN_ROOT_VAR, 0o755); err != nil { return nil, err }`,
			`$INFO_VAR, err := os.Stat($CLEAN_ROOT_VAR)`,
			`if err != nil { return nil, err }`,
			`if !$INFO_VAR.IsDir() { return nil, fmt.Errorf("MONOLIFT_FILESYSTEM_ROOT %s is not a directory", $CLEAN_ROOT_VAR) }`,
		},
		ConstructorLines: []string{
			`$RESOURCE_VAR, err := filesystem.NewLocal($CLEAN_ROOT_VAR)`,
			`if err != nil { return nil, err }`,
			`state.$STATE_FIELD = $RESOURCE_VAR`,
		},
		CloseSource: "state.$STATE_FIELD.Close()",
		ExtractedEnvVars: []EnvVar{
			{Name: "MONOLIFT_FILESYSTEM_ROOT", Value: "/monolift/durable"},
		},
		SharedVolumeMounts: []SharedVolumeMount{{
			Name:           "monolift-durable-root",
			ClaimName:      "${SERVICE}-durable-root",
			MountPath:      "/monolift/durable",
			StorageRequest: "1Gi",
		}},
		RootRelativePathSuffixes: []string{"Key"},
	}
}

// contextReconstructor detects context.Context parameters and produces a
// reconstructor that creates context.Background() on the server side.
// Context is never serialized across the boundary.
func contextReconstructor(typ types.Type) (Reconstructor, bool) {
	named := namedType(typ)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return Reconstructor{}, false
	}
	if named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context" {
		return Reconstructor{
			ID:      "context_background",
			Type:    typeString(typ, ""),
			Imports: []string{"context"},
		}, true
	}
	return Reconstructor{}, false
}

// loggerInterfaceReconstructor detects logger interface parameters (such as
// mlog.LoggerIFace) and produces a reconstructor that assigns nil on the
// server side.  Logger interfaces are not serialized across the boundary.
func loggerInterfaceReconstructor(typ types.Type) (Reconstructor, bool) {
	named := namedType(typ)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return Reconstructor{}, false
	}
	if _, ok := named.Underlying().(*types.Interface); !ok {
		return Reconstructor{}, false
	}
	typeName := named.Obj().Name()
	if strings.Contains(typeName, "Logger") {
		pkgPath := named.Obj().Pkg().Path()
		return Reconstructor{
			ID:      "discard_logger",
			Type:    typeString(typ, ""),
			Imports: []string{pkgPath},
		}, true
	}
	return Reconstructor{}, false
}

func sqlWrapperReconstructor(typ types.Type) (Reconstructor, bool) {
	named := namedType(typ)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return Reconstructor{}, false
	}
	pkgPath := named.Obj().Pkg().Path()
	typeName := named.Obj().Name()
	entry, ok := sqlWrapperRegistry[pkgPath+"."+typeName]
	if !ok || entry.ConstructorFunc == "" {
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
			constructorPkg := packageAlias(pkgPath)
			constructorFunc := entry.ConstructorFunc
			constructorArgOrder := append([]string(nil), entry.ConstructorArgOrder...)
			if len(constructorArgOrder) == 0 {
				constructorArgOrder = []string{"db"}
			}
			return Reconstructor{
				ID:                      "sql_db_wrapper",
				Type:                    typeString(typ, ""),
				Imports:                 []string{"context", "database/sql", "os", "_ github.com/lib/pq", pkgPath},
				ConstructorPkg:          constructorPkg,
				ConstructorFunc:         constructorFunc,
				ConstructorArgOrder:     constructorArgOrder,
				ConstructorPackagePath:  pkgPath,
				ConstructorPackageAlias: constructorPkg,
				ConstructorName:         constructorFunc,
				CloseSource:             "db.Close()",
			}, true
		}
	}
	return Reconstructor{}, false
}

func planReconstructors(plan *Plan) []Reconstructor {
	if plan == nil {
		return nil
	}
	reconstructors := make([]Reconstructor, 0, len(plan.ReconstructedParams)+1)
	for _, param := range plan.ReconstructedParams {
		if param.Reconstructor.ID != "" {
			reconstructors = append(reconstructors, param.Reconstructor)
		}
	}
	if plan.ReceiverParam != nil && plan.ReceiverParam.Policy == ReceiverReconstructed && plan.ReceiverParam.Reconstructor.ID != "" {
		reconstructors = append(reconstructors, plan.ReceiverParam.Reconstructor)
	}
	return reconstructors
}

func reconstructedReceiverParam(plan *Plan) (ReconstructedParam, bool) {
	if plan == nil || plan.ReceiverParam == nil || plan.ReceiverParam.Policy != ReceiverReconstructed {
		return ReconstructedParam{}, false
	}
	baseType := strings.TrimPrefix(plan.ReceiverParam.GoType, "*")
	qualified := plan.CutPoint.PackageName + "." + baseType
	if plan.ReceiverParam.IsPointer {
		qualified = "*" + qualified
	}
	return ReconstructedParam{
		Param: Param{
			Name:             "receiver",
			GoType:           plan.ReceiverParam.GoType,
			QualifiedGoType:  qualified,
			TypePackagePath:  plan.CutPoint.PackagePath,
			TypePackageAlias: plan.CutPoint.PackageName,
		},
		Reconstructor: plan.ReceiverParam.Reconstructor,
	}, true
}
