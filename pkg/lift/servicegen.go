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

type ServerTemplateData struct {
	InterfacePackageAlias string // e.g., "userservice"
	InterfacePackagePath  string // e.g., "github.com/tgoodwin/monolift/demo/monolith/userservice"
	InterfaceTypeName     string // e.g., "Service"
	ServerStructName      string // e.g., "userServiceServer"
	DelegateFieldName     string // e.g., "userServiceDelegate"
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
