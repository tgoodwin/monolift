package lift

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/yaml"
)

//go:embed templates/service.yaml.tmpl
var serviceTemplate string

//go:embed templates/deployment.yaml.tmpl
var deploymentTemplate string

//go:embed templates/entrypoint-service.yaml.tmpl
var entrypointServiceTemplate string

const entrypointDeploymentName = "entrypoint"
const entrypointServiceName = "entrypoint-service"

const extractedServicePort = 8080 // Default port for extracted services

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

// EntrypointServiceTemplateData holds data for the entrypoint Service.
type EntrypointServiceTemplateData struct {
	ServiceName    string
	DeploymentName string
	Namespace      string
	TargetPort     int
}

// GenerateExtractedServiceManifests creates Kubernetes Service and Deployment manifests for an extracted service.
func GenerateExtractedServiceManifests(outputDir, serviceName, namespace, imageName string, envVars []EnvVar) error {
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
		TargetPort:  extractedServicePort, // Service targetPort should match extractedServicePort
	}
	deploymentData := DeploymentTemplateData{
		ServiceName:   serviceName,
		Namespace:     namespace,
		ImageName:     imageName,
		ContainerPort: extractedServicePort,
		EnvVars:       envVars,
	}

	k8sOutputDir := filepath.Join(outputDir, "k8s")
	if err := os.MkdirAll(k8sOutputDir, 0755); err != nil {
		return fmt.Errorf("creating k8s output directory %s: %w", k8sOutputDir, err)
	}

	// Generate Service manifest
	serviceFilePath := filepath.Join(k8sOutputDir, fmt.Sprintf("%s-service.yaml", serviceName))
	serviceFile, err := os.Create(serviceFilePath)
	if err != nil {
		return fmt.Errorf("creating service.yaml for %s: %w", serviceName, err)
	}
	defer serviceFile.Close()
	if err := serviceTmpl.Execute(serviceFile, serviceData); err != nil {
		return fmt.Errorf("executing service template for %s: %w", serviceName, err)
	}

	// Generate Deployment manifest
	deploymentFilePath := filepath.Join(k8sOutputDir, fmt.Sprintf("%s-deployment.yaml", serviceName))
	deploymentFile, err := os.Create(deploymentFilePath)
	if err != nil {
		return fmt.Errorf("creating deployment.yaml for %s: %w", serviceName, err)
	}
	defer deploymentFile.Close()
	return deploymentTmpl.Execute(deploymentFile, deploymentData)
}

// GenerateEntrypointManifests creates K8s Deployment and Service for the rewritten entrypoint.
// It reuses the original deployment manifest and just updates the image and labels.
func GenerateEntrypointManifests(outputDir, entrypointImageName string, originalManifestData []byte) error {
	// --- Step 1: Generate the Deployment manifest ---
	// We read the original manifest, modify it in memory, and write it back out.
	// This is more robust than templating as it preserves all original settings (volumes, probes, etc.).
	sch := runtime.NewScheme()
	if err := appsv1.AddToScheme(sch); err != nil {
		return fmt.Errorf("failed to add apps/v1 to scheme: %w", err)
	}
	deserializer := serializer.NewCodecFactory(sch).UniversalDeserializer()

	obj, gvk, err := deserializer.Decode(originalManifestData, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to decode original Kubernetes manifest: %w", err)
	}
	if gvk.Group != "apps" || gvk.Kind != "Deployment" {
		return fmt.Errorf("original manifest is not an apps/v1 Deployment (found %s/%s)", gvk.Group, gvk.Kind)
	}
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return fmt.Errorf("decoded object is not an *appsv1.Deployment: %T", obj)
	}

	// Extract namespace from the original manifest
	namespace := deployment.ObjectMeta.Namespace
	if namespace == "" {
		// If no namespace is specified, use "default"
		namespace = "default"
	}

	// Modify the deployment for the new entrypoint
	deployment.ObjectMeta.Name = entrypointDeploymentName
	if deployment.ObjectMeta.Labels == nil {
		deployment.ObjectMeta.Labels = make(map[string]string)
	}
	deployment.ObjectMeta.Labels["app.kubernetes.io/name"] = entrypointDeploymentName

	if deployment.Spec.Selector == nil {
		return fmt.Errorf("original deployment manifest has no spec.selector, which is required")
	}
	if deployment.Spec.Selector.MatchLabels == nil {
		deployment.Spec.Selector.MatchLabels = make(map[string]string)
	}
	deployment.Spec.Selector.MatchLabels["app.kubernetes.io/name"] = entrypointDeploymentName

	if deployment.Spec.Template.ObjectMeta.Labels == nil {
		deployment.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	}
	deployment.Spec.Template.ObjectMeta.Labels["app.kubernetes.io/name"] = entrypointDeploymentName

	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("original deployment has no containers")
	}
	deployment.Spec.Template.Spec.Containers[0].Image = entrypointImageName

	// Extract the container port from the original deployment
	if len(deployment.Spec.Template.Spec.Containers[0].Ports) == 0 {
		return fmt.Errorf("original deployment has no container ports defined")
	}
	targetPort := deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort

	// Marshal the modified deployment back to YAML
	modifiedDeploymentYAML, err := yaml.Marshal(deployment)
	if err != nil {
		return fmt.Errorf("failed to marshal modified entrypoint deployment: %w", err)
	}

	k8sOutputDir := filepath.Join(outputDir, "k8s")
	if err := os.MkdirAll(k8sOutputDir, 0755); err != nil {
		return fmt.Errorf("creating k8s output directory %s: %w", k8sOutputDir, err)
	}
	deploymentFilePath := filepath.Join(k8sOutputDir, "entrypoint-deployment.yaml")
	if err := os.WriteFile(deploymentFilePath, modifiedDeploymentYAML, 0644); err != nil {
		return fmt.Errorf("failed to write entrypoint deployment.yaml: %w", err)
	}

	// --- Step 2: Generate the Service manifest ---
	serviceTmpl, err := template.New("entrypoint-service").Parse(entrypointServiceTemplate)
	if err != nil {
		return fmt.Errorf("parsing entrypoint service template: %w", err)
	}

	serviceData := EntrypointServiceTemplateData{
		ServiceName:    entrypointServiceName,
		DeploymentName: entrypointDeploymentName,
		Namespace:      namespace,
		TargetPort:     int(targetPort),
	}

	serviceFilePath := filepath.Join(k8sOutputDir, "entrypoint-service.yaml")
	serviceFile, err := os.Create(serviceFilePath)
	if err != nil {
		return fmt.Errorf("creating entrypoint service.yaml: %w", err)
	}
	defer serviceFile.Close()

	return serviceTmpl.Execute(serviceFile, serviceData)
}
