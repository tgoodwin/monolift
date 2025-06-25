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

// ClientMethodConfig holds information about a single interface method for client generation.
type ClientMethodConfig struct {
	Name              string // e.g., "Register"
	HTTPRoute         string // e.g., "/register"
	FullSignature     string // e.g., "Register(ctx context.Context, req userTypes.RegisterReq) (userTypes.RegisterResp, error)"
	RequestArgName    string // e.g., "req"
	ResponseType      string // e.g., "userTypes.RegisterResp"
	ResponseZeroValue string // e.g., "userTypes.RegisterResp{}" or "nil"
}

// ClientTemplateData holds all information needed for generating the client.
type ClientTemplateData struct {
	PackageName           string // e.g., "userservice" (the client package name)
	ClientStructName      string // e.g., "client"
	InterfacePackageAlias string // e.g., "userservice" (the original interface package alias)
	InterfacePackagePath  string // e.g., "github.com/tgoodwin/monolift/demo/monolith/userservice"
	InterfaceTypeName     string // e.g., "Service"
	Methods               []ClientMethodConfig
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

// GetInterfaceClientMethodConfigs extracts method information from an interface for client template generation.
func GetInterfaceClientMethodConfigs(iface *types.Interface, qualifier types.Qualifier, imports map[string]string) ([]ClientMethodConfig, error) {
	var methods []ClientMethodConfig

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

		// Get request and response types
		reqArg := sig.Params().At(1)
		respResult := sig.Results().At(0)
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

		methods = append(methods, ClientMethodConfig{
			Name:              method.Name(),
			HTTPRoute:         "/" + strings.ToLower(method.Name()),
			FullSignature:     fullSig,
			RequestArgName:    reqArg.Name(),
			ResponseType:      respTypeString,
			ResponseZeroValue: respZeroValue,
		})
	}
	return methods, nil
}

// GetClientTemplateData gathers all necessary information to generate a client for a given interface.
func GetClientTemplateData(ifaceNameIdent *ast.Ident, definingPkg *packages.Package) (*ClientTemplateData, error) {
	ifaceObj := definingPkg.TypesInfo.Defs[ifaceNameIdent]
	ifaceTypeName, ok := ifaceObj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("object for %s is not a TypeName", ifaceNameIdent.Name)
	}
	iface, ok := ifaceTypeName.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("type for %s is not an Interface", ifaceNameIdent.Name)
	}

	imports := make(map[string]string)
	qualifier := func(p *types.Package) string {
		// The generated client is in the 'main' package, so it must always qualify types from other packages.
		if p.Path() == "builtin" {
			return "" // Don't qualify built-in types
		}
		// Add the package to our import list if it's not already there.
		if _, exists := imports[p.Path()]; !exists {
			imports[p.Path()] = util.DetermineImportAlias(p.Path(), p.Name())
		}
		// Return the package's declared name for use in the type string.
		return p.Name()
	}

	methodConfigs, err := GetInterfaceClientMethodConfigs(iface, qualifier, imports)
	if err != nil {
		return nil, err
	}

	// The client file will be part of the 'main' package in the entrypoint dir.
	// The template will set the package name to 'main'.
	// Here, we use the original package name for the file name.
	clientPackageName := definingPkg.Name

	data := &ClientTemplateData{
		PackageName:           clientPackageName, // Used for filename, e.g., "userservice_client.go"
		ClientStructName:      "client",          // A simple, unexported name.
		InterfacePackageAlias: definingPkg.Name,
		InterfacePackagePath:  definingPkg.PkgPath,
		InterfaceTypeName:     ifaceNameIdent.Name,
		Methods:               methodConfigs,
		Imports:               imports,
	}

	return data, nil
}
