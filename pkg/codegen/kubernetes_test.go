package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderKubernetesSanitizeHTMLGolden(t *testing.T) {
	plan := sanitizeHTMLDeployPlan(t)
	files, err := RenderKubernetes(plan)
	if err != nil {
		t.Fatal(err)
	}
	goldens := map[string][]byte{
		filepath.Join("testdata", "sanitizehtml_extracted_deployment.yaml.golden"): files[plan.ExtractedDeploymentPath],
		filepath.Join("testdata", "sanitizehtml_extracted_service.yaml.golden"):    files[plan.ExtractedServicePath],
		filepath.Join("testdata", "sanitizehtml_host_deployment.yaml.golden"):      files[plan.HostDeploymentPath],
		filepath.Join("testdata", "sanitizehtml_host_service.yaml.golden"):         files[plan.HostServicePath],
	}
	for goldenPath, got := range goldens {
		if os.Getenv("MONOLIFT_UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(goldenPath, got, 0644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("rendered YAML does not match %s", goldenPath)
		}
	}
}

func TestRenderKubernetesParsedShape(t *testing.T) {
	plan := sanitizeHTMLDeployPlan(t)
	files, err := RenderKubernetes(plan)
	if err != nil {
		t.Fatal(err)
	}
	extractedDeployment := parseKubernetesDoc(t, files[plan.ExtractedDeploymentPath])
	if extractedDeployment.APIVersion != "apps/v1" || extractedDeployment.Kind != "Deployment" {
		t.Fatalf("extracted deployment type = %s/%s", extractedDeployment.APIVersion, extractedDeployment.Kind)
	}
	if extractedDeployment.Metadata.Name != plan.Deploy.ExtractedServiceName {
		t.Fatalf("extracted deployment name = %s", extractedDeployment.Metadata.Name)
	}
	extractedContainer := extractedDeployment.Spec.Template.Spec.Containers[0]
	if extractedContainer.Name != "extracted" || extractedContainer.Image != plan.Deploy.ExtractedImage {
		t.Fatalf("extracted container = %+v", extractedContainer)
	}
	if extractedContainer.Ports[0].ContainerPort != 8081 || extractedContainer.ReadinessProbe.HTTPGet.Path != "/healthz" {
		t.Fatalf("extracted readiness/port = %+v", extractedContainer)
	}

	extractedService := parseKubernetesDoc(t, files[plan.ExtractedServicePath])
	if extractedService.Kind != "Service" || extractedService.Metadata.Name != plan.Deploy.ExtractedServiceName {
		t.Fatalf("extracted service = %+v", extractedService)
	}
	if extractedService.Spec.Type != "ClusterIP" || extractedService.Spec.Ports[0].Port != 8081 {
		t.Fatalf("extracted service spec = %+v", extractedService.Spec)
	}
	if extractedService.Spec.Selector.App != plan.Deploy.ExtractedServiceName {
		t.Fatalf("extracted service selector = %+v", extractedService.Spec.Selector)
	}

	hostDeployment := parseKubernetesDoc(t, files[plan.HostDeploymentPath])
	if hostDeployment.Kind != "Deployment" || hostDeployment.Metadata.Name != plan.Deploy.HostServiceName {
		t.Fatalf("host deployment = %+v", hostDeployment)
	}
	hostContainer := hostDeployment.Spec.Template.Spec.Containers[0]
	if hostContainer.Image != plan.Deploy.HostImage || hostContainer.Ports[0].ContainerPort != plan.Deploy.HostPort {
		t.Fatalf("host container = %+v", hostContainer)
	}
	if hostContainer.ReadinessProbe.HTTPGet.Path != plan.Deploy.HostReadinessPath {
		t.Fatalf("host readiness = %+v", hostContainer.ReadinessProbe)
	}
	env := map[string]string{}
	for _, item := range hostContainer.Env {
		env[item.Name] = item.Value
	}
	if env["MONOLIFT_LIFT_SANITIZEHTML"] != "on" ||
		env["MONOLIFT_LIFT_FAILMODE"] != "closed" ||
		env["MONOLIFT_SANITIZEHTML_ENDPOINT"] != "http://monolift-extracted-sanitizehtml:8081/invoke" ||
		env["DATABASE_URL"] == "" {
		t.Fatalf("host env = %+v", env)
	}

	hostService := parseKubernetesDoc(t, files[plan.HostServicePath])
	if hostService.Kind != "Service" || hostService.Spec.Ports[0].Port != plan.Deploy.HostPort {
		t.Fatalf("host service = %+v", hostService)
	}
	if hostService.Spec.Selector.App != plan.Deploy.HostServiceName {
		t.Fatalf("host service selector = %+v", hostService.Spec.Selector)
	}
}

func TestExtractedDeploymentOmitsLiftEnv(t *testing.T) {
	plan := sanitizeHTMLDeployPlan(t)
	files, err := RenderKubernetes(plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(files[plan.ExtractedDeploymentPath]), "MONOLIFT_LIFT_") {
		t.Fatalf("extracted deployment contains lift env:\n%s", files[plan.ExtractedDeploymentPath])
	}
}

type kubernetesDoc struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Type     string `yaml:"type"`
		Replicas int    `yaml:"replicas"`
		Selector struct {
			App         string            `yaml:"app"`
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"selector"`
		Template struct {
			Metadata struct {
				Labels map[string]string `yaml:"labels"`
			} `yaml:"metadata"`
			Spec struct {
				Containers []containerDoc `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
		Ports []servicePortDoc `yaml:"ports"`
	} `yaml:"spec"`
}

type containerDoc struct {
	Name            string             `yaml:"name"`
	Image           string             `yaml:"image"`
	ImagePullPolicy string             `yaml:"imagePullPolicy"`
	Ports           []containerPortDoc `yaml:"ports"`
	ReadinessProbe  readinessProbeDoc  `yaml:"readinessProbe"`
	Env             []envDoc           `yaml:"env"`
}

type containerPortDoc struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"containerPort"`
}

type servicePortDoc struct {
	Name       string `yaml:"name"`
	Port       int    `yaml:"port"`
	TargetPort int    `yaml:"targetPort"`
}

type readinessProbeDoc struct {
	HTTPGet struct {
		Path string `yaml:"path"`
		Port int    `yaml:"port"`
	} `yaml:"httpGet"`
	PeriodSeconds int `yaml:"periodSeconds"`
}

type envDoc struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func parseKubernetesDoc(t *testing.T, data []byte) kubernetesDoc {
	t.Helper()
	var doc kubernetesDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}
