package lift

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates/service.yaml.tmpl
var serviceTemplate string

//go:embed templates/deployment.yaml.tmpl
var deploymentTemplate string

// EnvVar represents an environment variable for a container.
type EnvVar struct {
	Name  string
	Value string
}

// ServiceTemplateData holds the data for the Kubernetes Service manifest.
type ServiceTemplateData struct {
	ServiceName string
	Namespace   string
	TargetPort  int
}

// DeploymentTemplateData holds the data for the Kubernetes Deployment manifest.
type DeploymentTemplateData struct {
	ServiceName   string
	Namespace     string
	ImageName     string
	ContainerPort int
	EnvVars       []EnvVar
}

// GenerateExtractedServiceManifests creates Kubernetes Service and Deployment manifests for an extracted service.
func GenerateExtractedServiceManifests(outputDir, serviceName, namespace, imageName string, containerPort int, envVars []EnvVar) error {
	serviceTmpl, err := template.New("service").Parse(serviceTemplate)
	if err != nil {
		return fmt.Errorf("parsing service template: %w", err)
	}
	deploymentTmpl, err := template.New("deployment").Parse(deploymentTemplate)
	if err != nil {
		return fmt.Errorf("parsing deployment template: %w", err)
	}

	serviceData := ServiceTemplateData{
		ServiceName: serviceName,
		Namespace:   namespace,
		TargetPort:  containerPort, // Service targetPort should match containerPort
	}
	deploymentData := DeploymentTemplateData{
		ServiceName:   serviceName,
		Namespace:     namespace,
		ImageName:     imageName,
		ContainerPort: containerPort,
		EnvVars:       envVars,
	}

	serviceOutputDir := filepath.Join(outputDir, serviceName)
	// Ensure the directory exists
	if err := os.MkdirAll(serviceOutputDir, 0755); err != nil {
		return fmt.Errorf("creating service output directory %s: %w", serviceOutputDir, err)
	}

	// Generate Service manifest
	serviceFilePath := filepath.Join(serviceOutputDir, "service.yaml")
	serviceFile, err := os.Create(serviceFilePath)
	if err != nil {
		return fmt.Errorf("creating service.yaml for %s: %w", serviceName, err)
	}
	defer serviceFile.Close()
	if err := serviceTmpl.Execute(serviceFile, serviceData); err != nil {
		return fmt.Errorf("executing service template for %s: %w", serviceName, err)
	}

	// Generate Deployment manifest
	deploymentFilePath := filepath.Join(serviceOutputDir, "deployment.yaml")
	deploymentFile, err := os.Create(deploymentFilePath)
	if err != nil {
		return fmt.Errorf("creating deployment.yaml for %s: %w", serviceName, err)
	}
	defer deploymentFile.Close()
	return deploymentTmpl.Execute(deploymentFile, deploymentData)
}
