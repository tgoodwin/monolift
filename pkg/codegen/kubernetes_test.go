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

func TestExtractedDeploymentIncludesDatabaseURLForSQLReconstructor(t *testing.T) {
	plan := sqlDeployPlan()
	files, err := RenderKubernetes(plan)
	if err != nil {
		t.Fatal(err)
	}
	extractedDeployment := parseKubernetesDoc(t, files[plan.ExtractedDeploymentPath])
	env := envMap(extractedDeployment.Spec.Template.Spec.Containers[0].Env)
	if got, want := env["DATABASE_URL"], "postgres://miniflux@postgres/miniflux?sslmode=disable"; got != want {
		t.Fatalf("DATABASE_URL = %q, want %q; env=%+v", got, want, env)
	}
	if _, ok := env["RUN_MIGRATIONS"]; ok {
		t.Fatalf("extracted deployment propagated host-only RUN_MIGRATIONS: %+v", env)
	}
	if _, ok := env["MONOLIFT_LIFT_QUERY"]; ok {
		t.Fatalf("extracted deployment propagated lift env: %+v", env)
	}
}

func TestExtractedDeploymentOmitsEnvBlockWithoutSQLReconstructor(t *testing.T) {
	plan := sanitizeHTMLDeployPlan(t)
	files, err := RenderKubernetes(plan)
	if err != nil {
		t.Fatal(err)
	}
	extractedDeployment := parseKubernetesDoc(t, files[plan.ExtractedDeploymentPath])
	if env := extractedDeployment.Spec.Template.Spec.Containers[0].Env; len(env) != 0 {
		t.Fatalf("extracted env = %+v, want none", env)
	}
	if strings.Contains(string(files[plan.ExtractedDeploymentPath]), "\n          env:\n") {
		t.Fatalf("extracted deployment rendered env block without SQL reconstructor:\n%s", files[plan.ExtractedDeploymentPath])
	}
}

func TestRenderKubernetesFilesystemReconstructorSharedRoot(t *testing.T) {
	plan := filesystemReceiverPlan()
	files, err := RenderKubernetes(plan)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SharedVolumeClaimPath == "" {
		t.Fatal("SharedVolumeClaimPath is empty")
	}
	if _, ok := files[plan.SharedVolumeClaimPath]; !ok {
		t.Fatalf("missing shared volume claim artifact %s", plan.SharedVolumeClaimPath)
	}

	extractedDeployment := parseKubernetesDoc(t, files[plan.ExtractedDeploymentPath])
	extractedContainer := extractedDeployment.Spec.Template.Spec.Containers[0]
	env := envMap(extractedContainer.Env)
	if got, want := env["MONOLIFT_FILESYSTEM_ROOT"], "/monolift/durable"; got != want {
		t.Fatalf("MONOLIFT_FILESYSTEM_ROOT = %q, want %q; env=%+v", got, want, env)
	}
	if !hasVolumeMount(extractedContainer.VolumeMounts, "monolift-durable-root", "/monolift/durable") {
		t.Fatalf("extracted volumeMounts = %+v", extractedContainer.VolumeMounts)
	}
	if !hasPVCVolume(extractedDeployment.Spec.Template.Spec.Volumes, "monolift-durable-root", "create-thumb-durable-root") {
		t.Fatalf("extracted volumes = %+v", extractedDeployment.Spec.Template.Spec.Volumes)
	}

	hostDeployment := parseKubernetesDoc(t, files[plan.HostDeploymentPath])
	hostContainer := hostDeployment.Spec.Template.Spec.Containers[0]
	if !hasVolumeMount(hostContainer.VolumeMounts, "monolift-durable-root", "/monolift/durable") {
		t.Fatalf("host volumeMounts = %+v", hostContainer.VolumeMounts)
	}
	if !hasPVCVolume(hostDeployment.Spec.Template.Spec.Volumes, "monolift-durable-root", "create-thumb-durable-root") {
		t.Fatalf("host volumes = %+v", hostDeployment.Spec.Template.Spec.Volumes)
	}

	claim := parseKubernetesDoc(t, files[plan.SharedVolumeClaimPath])
	if claim.Kind != "PersistentVolumeClaim" || claim.Metadata.Name != "create-thumb-durable-root" {
		t.Fatalf("claim = %+v", claim)
	}
}

func hasVolumeMount(mounts []volumeMountDoc, name, path string) bool {
	for _, mount := range mounts {
		if mount.Name == name && mount.MountPath == path {
			return true
		}
	}
	return false
}

func hasPVCVolume(volumes []volumeDoc, name, claimName string) bool {
	for _, volume := range volumes {
		if volume.Name == name && volume.PersistentVolumeClaim.ClaimName == claimName {
			return true
		}
	}
	return false
}

func sqlDeployPlan() *Plan {
	plan := &Plan{
		ServiceName:      "query",
		EnvServiceName:   "QUERY",
		SourceModuleRoot: "/tmp/source",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/query",
			PackageName: "query",
			FuncName:    "Run",
		},
		ReconstructedParams: []ReconstructedParam{{
			Param: Param{
				Name:            "db",
				GoType:          "*sql.DB",
				QualifiedGoType: "*sql.DB",
				Codec:           CodecJSON,
			},
			Reconstructor: Reconstructor{ID: "sql_db"},
		}},
	}
	applyLiftOptions(plan, LiftOptions{
		Output: filepath.Join(plan.SourceModuleRoot, ".monolift-query"),
		Deploy: DeployOptions{
			HostServiceName:      "query-host",
			ExtractedServiceName: "query",
			HostEnvVars: []EnvVar{
				{Name: "DATABASE_URL", Value: "postgres://miniflux@postgres/miniflux?sslmode=disable"},
				{Name: "RUN_MIGRATIONS", Value: "1"},
			},
		},
	})
	return plan
}

func envMap(env []envDoc) map[string]string {
	out := map[string]string{}
	for _, item := range env {
		out[item.Name] = item.Value
	}
	return out
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
				Volumes    []volumeDoc    `yaml:"volumes"`
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
	VolumeMounts    []volumeMountDoc   `yaml:"volumeMounts"`
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

type volumeMountDoc struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
}

type volumeDoc struct {
	Name                  string `yaml:"name"`
	PersistentVolumeClaim struct {
		ClaimName string `yaml:"claimName"`
	} `yaml:"persistentVolumeClaim"`
}

func parseKubernetesDoc(t *testing.T, data []byte) kubernetesDoc {
	t.Helper()
	var doc kubernetesDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}
