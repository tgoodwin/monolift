package lift

import (
	"fmt"
	"os"
	"text/template"

	_ "embed"

	util "github.com/tgoodwin/monolift/pkg/util"
)

//go:embed templates/server.go.tmpl
var serverTemplate string

// MethodData holds information about a single interface method for template generation.
type MethodData struct {
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
	Methods               []MethodData      // List of methods to generate handlers for
	Imports               map[string]string // To collect necessary imports: map[alias]path
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
