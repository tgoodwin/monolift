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

//go:embed templates/delegate.go.tmpl
var delegateTemplate string

// DelegateTemplateData holds all information needed for generating the delegate client.
type DelegateTemplateData struct {
	PackageName            string // e.g., "main"
	DelegateStructName     string // e.g., "UserServiceDelegate"
	InterfacePackageAlias  string // e.g., "userservice"
	InterfacePackagePath   string // e.g., "github.com/tgoodwin/monolift/demo/monolith/userservice"
	InterfaceTypeName      string // e.g., "Service"
	RemoteClientStructName string // e.g., "client"
	Methods                []ClientMethodConfig
	Imports                map[string]string
}

// ExecuteDelegateTemplate generates the delegate client code for a service.
func ExecuteDelegateTemplate(entrypointDir string, data DelegateTemplateData) error {
	// Generate a file like "userservice_delegate.go" inside the entrypoint directory.
	outfile := filepath.Join(entrypointDir, fmt.Sprintf("%s_delegate.go", data.InterfacePackageAlias))
	file, err := os.Create(outfile)
	if err != nil {
		return fmt.Errorf("error creating delegate output file %s: %w", outfile, err)
	}
	defer file.Close()

	// The package name for this generated file should be "main", as it's part of the entrypoint package.
	data.PackageName = "main"

	funcMap := template.FuncMap{
		"join": strings.Join,
	}

	tmpl, err := template.New("delegate").Funcs(funcMap).Parse(delegateTemplate)
	if err != nil {
		return fmt.Errorf("error parsing delegate template: %w", err)
	}
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("error executing delegate template: %w", err)
	}

	return util.GenerateImports(outfile)
}

// GetDelegateTemplateData gathers all necessary information to generate a delegate for a given interface.
func GetDelegateTemplateData(ifaceNameIdent *ast.Ident, definingPkg *packages.Package) (*DelegateTemplateData, error) {
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
		if p.Path() == "builtin" {
			return ""
		}
		if _, exists := imports[p.Path()]; !exists {
			imports[p.Path()] = util.DetermineImportAlias(p.Path(), p.Name())
		}
		return p.Name()
	}

	methodConfigs, err := GetInterfaceClientMethodConfigs(iface, qualifier, imports)
	if err != nil {
		return nil, err
	}

	data := &DelegateTemplateData{
		DelegateStructName:     ifaceNameIdent.Name + "ClientDelegate",
		RemoteClientStructName: "client", // This is hardcoded in client.go.tmpl
		InterfacePackageAlias:  definingPkg.Name,
		InterfacePackagePath:   definingPkg.PkgPath,
		InterfaceTypeName:      ifaceNameIdent.Name,
		Methods:                methodConfigs,
		Imports:                imports,
	}

	return data, nil
}
