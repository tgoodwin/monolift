package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/extract/bootpath"
	corev1 "k8s.io/api/core/v1"
)

func TestMattermostDatasourceRendersSecretRef(t *testing.T) {
	env, configMap, secret, _, _ := RenderConfig(bootpath.BootSpec{
		ConfigSources: []bootpath.ConfigSource{
			bootpath.EnvSource{Name: "MM_SQLSETTINGS_DATASOURCE", Default: "postgres://plaintext"},
		},
	})

	if configMap != nil {
		if _, ok := configMap.Data["MM_SQLSETTINGS_DATASOURCE"]; ok {
			t.Fatalf("datasource rendered to ConfigMap: %#v", configMap.Data)
		}
	}
	if secret == nil || secret.StringData["MM_SQLSETTINGS_DATASOURCE"] != "postgres://plaintext" {
		t.Fatalf("datasource missing from Secret: %#v", secret)
	}
	if len(env) != 1 || env[0].ValueFrom == nil || env[0].ValueFrom.SecretKeyRef == nil {
		t.Fatalf("datasource env var did not use SecretKeyRef: %#v", env)
	}
	if env[0].Value != "" || strings.Contains(mustJSON(t, env), "postgres://plaintext") {
		t.Fatalf("datasource leaked plaintext into env entries: %s", mustJSON(t, env))
	}
}

func TestRenderConfigGolden(t *testing.T) {
	boot := bootpath.BootSpec{
		ConfigSources: []bootpath.ConfigSource{
			bootpath.EnvSource{Name: "PUBLIC_HOST", Default: "localhost"},
			bootpath.EnvSource{Name: "API_TOKEN", Default: "redacted"},
			bootpath.FlagSource{Name: "config", Default: "config.json", FlagSet: "flag.CommandLine"},
			bootpath.FileSource{Path: "/etc/mattermost/config.json", Format: bootpath.FileFormatJSON, MountName: "config"},
			bootpath.LiteralSource{Value: "baked"},
			bootpath.DBSource{Name: "sql", QueryShape: "QueryRow", Required: true},
		},
	}
	env, configMap, secret, mounts, args := RenderConfig(boot)
	got := goldenView{
		Env:           simplifyEnv(env),
		ConfigMapData: cloneMap(configMap.Data),
		SecretData:    cloneMap(secret.StringData),
		VolumeMounts:  simplifyMounts(mounts),
		Args:          args,
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	data = append(data, '\n')
	want, err := os.ReadFile(filepath.Join("testdata", "render_config.golden.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != string(want) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", want, data)
	}
}

func TestRenderConfigStable(t *testing.T) {
	boot := bootpath.BootSpec{
		ConfigSources: []bootpath.ConfigSource{
			bootpath.FlagSource{Name: "zeta", Default: "z"},
			bootpath.EnvSource{Name: "ALPHA", Default: "a"},
			bootpath.EnvSource{Name: "DB_PASSWORD", Default: "secret"},
		},
	}
	first := renderJSON(t, boot)
	second := renderJSON(t, boot)
	if first != second {
		t.Fatalf("RenderConfig is unstable\nfirst: %s\nsecond: %s", first, second)
	}
}

func TestRenderConfigOmitsCompilerLiftEnv(t *testing.T) {
	env, configMap, secret, _, _ := RenderConfig(bootpath.BootSpec{
		ConfigSources: []bootpath.ConfigSource{
			bootpath.EnvSource{Name: "MONOLIFT_LIFT_CLEANPATH_URL", Default: "http://loop"},
			bootpath.EnvSource{Name: "SERVICE_HOST", Default: "localhost"},
		},
	})
	rendered := mustJSON(t, goldenView{
		Env:           simplifyEnv(env),
		ConfigMapData: cloneMap(configMap.Data),
		SecretData:    cloneSecretData(secret),
	})
	if strings.Contains(rendered, "MONOLIFT_LIFT_") {
		t.Fatalf("compiler lift env leaked into rendered config: %s", rendered)
	}
}

type goldenView struct {
	Env           []envView         `json:"env"`
	ConfigMapData map[string]string `json:"configMapData,omitempty"`
	SecretData    map[string]string `json:"secretData,omitempty"`
	VolumeMounts  []mountView       `json:"volumeMounts,omitempty"`
	Args          []string          `json:"args,omitempty"`
}

type envView struct {
	Name string `json:"name"`
	From string `json:"from"`
	Key  string `json:"key"`
}

type mountView struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	SubPath   string `json:"subPath"`
	ReadOnly  bool   `json:"readOnly"`
}

func simplifyEnv(env []corev1.EnvVar) []envView {
	out := make([]envView, 0, len(env))
	for _, item := range env {
		view := envView{Name: item.Name}
		if item.ValueFrom != nil && item.ValueFrom.ConfigMapKeyRef != nil {
			view.From = item.ValueFrom.ConfigMapKeyRef.Name
			view.Key = item.ValueFrom.ConfigMapKeyRef.Key
		}
		if item.ValueFrom != nil && item.ValueFrom.SecretKeyRef != nil {
			view.From = item.ValueFrom.SecretKeyRef.Name
			view.Key = item.ValueFrom.SecretKeyRef.Key
		}
		out = append(out, view)
	}
	return out
}

func simplifyMounts(mounts []corev1.VolumeMount) []mountView {
	out := make([]mountView, 0, len(mounts))
	for _, mount := range mounts {
		out = append(out, mountView{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
		})
	}
	return out
}

func renderJSON(t *testing.T, boot bootpath.BootSpec) string {
	t.Helper()
	env, configMap, secret, mounts, args := RenderConfig(boot)
	return mustJSON(t, goldenView{
		Env:           simplifyEnv(env),
		ConfigMapData: cloneConfigMapData(configMap),
		SecretData:    cloneSecretData(secret),
		VolumeMounts:  simplifyMounts(mounts),
		Args:          args,
	})
}

func cloneConfigMapData(configMap *corev1.ConfigMap) map[string]string {
	if configMap == nil {
		return nil
	}
	return cloneMap(configMap.Data)
}

func cloneSecretData(secret *corev1.Secret) map[string]string {
	if secret == nil {
		return nil
	}
	return cloneMap(secret.StringData)
}

func cloneMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	return string(data)
}
