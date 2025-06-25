package lift

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"strings"
	"text/template"

	_ "embed"

	util "github.com/tgoodwin/monolift/pkg/util"
	"golang.org/x/tools/go/packages"
)

//go:embed templates/server.go.tmpl
var serverTemplate string

// MethodConfig holds information about a single interface method for template generation.
type MethodConfig struct {
	Name            string // Original method name, e.g., "Register"
	HandlerFuncName string // Name for the HTTP handler func, e.g., "handleRegister"
	HTTPRoute       string // HTTP route, e.g., "/register"
	// We will add more fields here later, like request/response types
}

// ServerTemplateData holds all information needed for generating the server.
type ServerTemplateData struct {
	InterfacePackageAlias string
	InterfacePackagePath  string
	InterfaceTypeName     string
	ServerStructName      string
	DelegateFieldName     string
	Methods               []MethodConfig    // List of methods to generate handlers for
	Imports               map[string]string // To collect necessary imports: map[alias]path
	PackageScopeDeps      []*Dependency
	FunctionScopeDeps     []*Dependency
	RootDependency        *Dependency
}

func ExecuteAndPrintTemplate(name, outputDir string, data ServerTemplateData) error {
	serviceOutputDir := fmt.Sprintf("%s/%s", outputDir, data.InterfacePackageAlias)
	os.RemoveAll(serviceOutputDir) // Clean up previous output
	if err := os.MkdirAll(serviceOutputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", serviceOutputDir, err)
	}

	// create the output file
	outfile := fmt.Sprintf("%s/main.go", serviceOutputDir)
	file, err := os.Create(outfile)
	if err != nil {
		fmt.Printf("Error creating output file %s: %v\n", outfile, err)
		return err
	}
	defer file.Close()

	// execute the template

	tmpl, err := template.New(name).Parse(serverTemplate)
	if err != nil {
		fmt.Printf("Error parsing template %s: %v\n", name, err)
		return err
	}
	if err := tmpl.Execute(file, data); err != nil {
		fmt.Printf("Error executing template %s: %v\n", name, err)
		return err
	}

	if err := util.GenerateImports(outfile); err != nil {
		fmt.Printf("Error generating imports in %s: %v\n", outfile, err)
		return err
	}

	if err := util.InitGoMod(data.InterfacePackageAlias, serviceOutputDir); err != nil {
		fmt.Printf("Error initializing go.mod in %s: %v\n", serviceOutputDir, err)
		return err
	}

	return nil
}

// GetInterfaceMethodConfigs extracts method information from an interface for template generation.
func GetInterfaceMethodConfigs(ifaceNameIdent *ast.Ident, pkg *packages.Package) ([]MethodConfig, error) {
	var methodDataList []MethodConfig

	ifaceObj := pkg.TypesInfo.Defs[ifaceNameIdent]
	if ifaceObj == nil {
		return nil, fmt.Errorf("could not find type object for interface %s", ifaceNameIdent.Name)
	}

	ifaceTypeName, ok := ifaceObj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("object for %s is not a TypeName", ifaceNameIdent.Name)
	}

	ifaceType, ok := ifaceTypeName.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("type for %s is not an Interface", ifaceNameIdent.Name)
	}

	for i := 0; i < ifaceType.NumExplicitMethods(); i++ {
		method := ifaceType.ExplicitMethod(i)
		methodName := method.Name()
		handlerFuncName := "handle" + strings.ToUpper(methodName[:1]) + methodName[1:]
		httpRoute := "/" + strings.ToLower(methodName)
		methodDataList = append(methodDataList, MethodConfig{Name: methodName, HandlerFuncName: handlerFuncName, HTTPRoute: httpRoute})
	}
	return methodDataList, nil
}
