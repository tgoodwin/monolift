package lift

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	_ "embed"

	"github.com/tgoodwin/monolift/pkg/util"
	"golang.org/x/tools/go/packages"
)

//go:embed templates/client.go.tmpl
var clientTemplate string

// ClientTemplateData holds all information needed for generating the client.
type ClientTemplateData struct {
	PackageName           string // e.g., "userservice" (the client package name)
	ClientStructName      string // e.g., "client"
	InterfacePackageAlias string // e.g., "userservice" (the original interface package alias)
	InterfacePackagePath  string // e.g., "github.com/tgoodwin/monolift/demo/monolith/userservice"
	InterfaceTypeName     string // e.g., "Service"
	Methods               []MethodConfig
	Imports               map[string]string
}

// ExecuteClientTemplate generates the client code for a service.
func ExecuteClientTemplate(entrypointDir string, data ClientTemplateData) error {
	// Generate a file like "userservice_client.go" inside the entrypoint directory.
	outfile := filepath.Join(entrypointDir, fmt.Sprintf("%s_client.go", data.PackageName))
	file, err := os.Create(outfile)
	if err != nil {
		return fmt.Errorf("error creating client output file %s: %w", outfile, err)
	}
	defer file.Close()

	// The package name for this generated file should be "main", as it's part of the entrypoint package.
	data.PackageName = "main"
	tmpl, err := template.New("client").Parse(clientTemplate)
	if err != nil {
		return fmt.Errorf("error parsing client template: %w", err)
	}
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("error executing client template: %w", err)
	}

	return util.GenerateImports(outfile)
}

// GetInterfaceMethodConfigs extracts method information from an interface for template generation.
func GetInterfaceMethodConfigs(iface *types.Interface, qualifier types.Qualifier) ([]MethodConfig, error) {
	var methods []MethodConfig

	for i := 0; i < iface.NumExplicitMethods(); i++ {
		method := iface.ExplicitMethod(i)
		sig, ok := method.Type().(*types.Signature)
		if !ok {
			continue // Should not happen for methods
		}

		// Basic validation: expecting (context.Context, req) (resp, error)
		if sig.Params().Len() != 2 || sig.Results().Len() != 2 {
			return nil, fmt.Errorf("method %s has an unsupported signature: expected 2 params and 2 results, got %d and %d", method.Name(), sig.Params().Len(), sig.Results().Len())
		}
		if !strings.HasSuffix(sig.Params().At(0).Type().String(), "context.Context") {
			return nil, fmt.Errorf("method %s has an unsupported signature: first parameter must be context.Context", method.Name())
		}
		if !strings.HasSuffix(sig.Results().At(1).Type().String(), "error") {
			return nil, fmt.Errorf("method %s has an unsupported signature: second result must be error", method.Name())
		}

		// Reconstruct the full signature string
		fullSig := types.TypeString(sig, qualifier)
		fullSig = strings.TrimPrefix(fullSig, "func") // remove "func" prefix
		fullSig = method.Name() + fullSig

		// Get all param names
		var paramNames []string
		for j := 0; j < sig.Params().Len(); j++ {
			param := sig.Params().At(j)
			paramNames = append(paramNames, param.Name())
		}

		// Get request and response types
		reqArg := sig.Params().At(1)
		respResult := sig.Results().At(0)
		reqTypeString := types.TypeString(reqArg.Type(), qualifier)
		respTypeString := types.TypeString(respResult.Type(), qualifier)

		// Determine zero value for the response type
		var respZeroValue string
		respType := respResult.Type()
		switch t := respType.(type) {
		case *types.Named:
			// If it's a named type, its zero value is TypeName{}
			// Unless its underlying type is a pointer, then its zero value is nil.
			if _, isPointer := t.Underlying().(*types.Pointer); isPointer {
				respZeroValue = "nil"
			} else {
				respZeroValue = fmt.Sprintf("%s{}", respTypeString)
			}
		case *types.Pointer, *types.Slice, *types.Map, *types.Interface, *types.Chan, *types.Signature: // These types (or their underlying types if not named) have nil as zero value.
			respZeroValue = "nil"
		default:
			panic("could not infer zero value for response type: " + respTypeString)
		}

		methods = append(methods, MethodConfig{
			Name:              method.Name(),
			HTTPRoute:         "/" + strings.ToLower(method.Name()),
			HandlerFuncName:   "handle" + method.Name(),
			FullSignature:     fullSig,
			RequestArgName:    reqArg.Name(),
			RequestType:       reqTypeString,
			ParamNames:        paramNames,
			ResponseType:      respTypeString,
			ResponseZeroValue: respZeroValue,
		})
	}
	return methods, nil
}

// GetMethodConfigsForInterface is a helper that wraps GetInterfaceMethodConfigs.
// It takes an interface name and its package, resolves the types, and returns the method configs.
func GetMethodConfigsForInterface(ifaceNameIdent *ast.Ident, definingPkg *packages.Package, imports map[string]string) ([]MethodConfig, error) {
	ifaceObj := definingPkg.TypesInfo.Defs[ifaceNameIdent]
	ifaceTypeName, ok := ifaceObj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("object for %s is not a TypeName", ifaceNameIdent.Name)
	}
	iface, ok := ifaceTypeName.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("type for %s is not an Interface", ifaceNameIdent.Name)
	}

	qualifier := func(p *types.Package) string {
		if p.Path() == "builtin" {
			return "" // Don't qualify built-in types
		}
		// Add the package to our import list if it's not already there.
		if _, exists := imports[p.Path()]; !exists {
			imports[p.Path()] = util.DetermineImportAlias(p.Path(), p.Name())
		}
		return p.Name()
	}

	return GetInterfaceMethodConfigs(iface, qualifier)
}

// GetClientTemplateData gathers all necessary information to generate a client for a given interface.
func GetClientTemplateData(ifaceNameIdent *ast.Ident, definingPkg *packages.Package) (*ClientTemplateData, error) {
	imports := make(map[string]string)
	methodConfigs, err := GetMethodConfigsForInterface(ifaceNameIdent, definingPkg, imports)
	if err != nil {
		return nil, err
	}

	clientPackageName := definingPkg.Name

	data := &ClientTemplateData{
		PackageName:           clientPackageName,
		ClientStructName:      clientPackageName + "Client", // n.b. delegategen relies on this naming scheme
		InterfacePackageAlias: definingPkg.Name,
		InterfacePackagePath:  definingPkg.PkgPath,
		InterfaceTypeName:     ifaceNameIdent.Name,
		Methods:               methodConfigs,
		Imports:               imports,
	}

	return data, nil
}
