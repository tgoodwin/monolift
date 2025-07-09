package compiler

import (
	"fmt"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/tgoodwin/monolift/pkg/lift"
)

// extractEnvVarsFromK8sManifest parses a Kubernetes Deployment manifest
// and extracts environment variables from the first container.
// It currently only supports simple value environment variables and skips ValueFrom.
func extractEnvVarsFromK8sManifest(manifestPath string) ([]lift.EnvVar, error) {
	if manifestPath == "" {
		return nil, nil // No manifest path provided, no env vars to extract
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Kubernetes manifest file %s: %w", manifestPath, err)
	}

	// Create a new scheme and add apps/v1 types to it
	sch := runtime.NewScheme()
	if err := appsv1.AddToScheme(sch); err != nil {
		return nil, fmt.Errorf("failed to add apps/v1 to scheme: %w", err)
	}

	// Create a CodecFactory and a universal deserializer that can handle any versioned object.
	deserializer := serializer.NewCodecFactory(sch).UniversalDeserializer()

	// Decode the YAML into a runtime.Object
	obj, gvk, err := deserializer.Decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Kubernetes manifest %s: %w", manifestPath, err)
	}

	// Check if the decoded object is an apps/v1 Deployment
	if gvk == nil || gvk.Group != "apps" || gvk.Kind != "Deployment" {
		return nil, fmt.Errorf("manifest %s is not an apps/v1 Deployment (found %s/%s)", manifestPath, gvk.Group, gvk.Kind)
	}

	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil, fmt.Errorf("decoded object is not an apps/v1 Deployment: %T", obj)
	}

	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return nil, fmt.Errorf("deployment %s has no containers defined", deployment.Name)
	}

	// Extract from the first container.
	container := deployment.Spec.Template.Spec.Containers[0]

	var extractedEnvVars []lift.EnvVar
	for _, env := range container.Env {
		if env.Value != "" {
			extractedEnvVars = append(extractedEnvVars, lift.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		} else if env.ValueFrom != nil {
			fmt.Printf("Warning: Environment variable '%s' in original manifest uses ValueFrom, which is not yet supported for extraction. Skipping.\n", env.Name)
		}
	}

	return extractedEnvVars, nil
}
