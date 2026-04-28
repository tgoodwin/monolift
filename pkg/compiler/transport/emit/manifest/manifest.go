package manifest

import (
	"sort"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/extract/bootpath"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configMapName = "monolift-boot-config"
	secretName    = "monolift-boot-secret"
)

var explicitSecretNames = map[string]bool{
	"MM_SQLSETTINGS_DATASOURCE": true,
}

// RenderConfig converts boot-path config evidence into deterministic Kubernetes
// manifest fragments for extracted-service deployments.
func RenderConfig(boot bootpath.BootSpec) ([]corev1.EnvVar, *corev1.ConfigMap, *corev1.Secret, []corev1.VolumeMount, []string) {
	sources := sortedSources(boot.ConfigSources)
	configData := map[string]string{}
	secretData := map[string]string{}
	var envEntries []corev1.EnvVar
	var volumeMounts []corev1.VolumeMount
	var args []string

	for _, source := range sources {
		switch typed := source.(type) {
		case bootpath.EnvSource:
			if isCompilerLiftEnv(typed.Name) {
				continue
			}
			if isSensitiveName(typed.Name) {
				secretData[typed.Name] = typed.Default
				envEntries = append(envEntries, secretEnvVar(typed.Name, typed.Name))
			} else {
				configData[typed.Name] = typed.Default
				envEntries = append(envEntries, configEnvVar(typed.Name, typed.Name))
			}
		case bootpath.FlagSource:
			key := envKeyForFlag(typed.Name)
			if isCompilerLiftEnv(key) {
				continue
			}
			if isSensitiveName(typed.Name) {
				secretData[key] = typed.Default
				envEntries = append(envEntries, secretEnvVar(key, key))
			} else {
				configData[key] = typed.Default
				envEntries = append(envEntries, configEnvVar(key, key))
			}
			args = append(args, "--"+typed.Name+"=$("+key+")")
		case bootpath.FileSource:
			key := fileKey(typed)
			if isSensitiveName(typed.Path) || isSensitiveName(typed.MountName) {
				secretData[key] = ""
			} else {
				configData[key] = ""
			}
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeName(typed),
				MountPath: typed.Path,
				SubPath:   key,
				ReadOnly:  true,
			})
		case bootpath.LiteralSource, bootpath.DBSource:
			continue
		}
	}

	sort.Slice(envEntries, func(i, j int) bool { return envEntries[i].Name < envEntries[j].Name })
	sort.Slice(volumeMounts, func(i, j int) bool {
		if volumeMounts[i].Name == volumeMounts[j].Name {
			return volumeMounts[i].MountPath < volumeMounts[j].MountPath
		}
		return volumeMounts[i].Name < volumeMounts[j].Name
	})
	sort.Strings(args)

	return envEntries, configMap(configData), secret(secretData), volumeMounts, args
}

func sortedSources(sources []bootpath.ConfigSource) []bootpath.ConfigSource {
	out := append([]bootpath.ConfigSource(nil), sources...)
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i].Kind() + ":" + out[i].Identifier()
		right := out[j].Kind() + ":" + out[j].Identifier()
		return left < right
	})
	return out
}

func configEnvVar(name, key string) corev1.EnvVar {
	optional := true
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				Key:                  key,
				Optional:             &optional,
			},
		},
	}
}

func secretEnvVar(name, key string) corev1.EnvVar {
	optional := true
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  key,
				Optional:             &optional,
			},
		},
	}
}

func configMap(data map[string]string) *corev1.ConfigMap {
	if len(data) == 0 {
		return nil
	}
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: configMapName},
		Data:       data,
	}
}

func secret(data map[string]string) *corev1.Secret {
	if len(data) == 0 {
		return nil
	}
	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: secretName},
		Type:       corev1.SecretTypeOpaque,
		StringData: data,
	}
}

func isSensitiveName(name string) bool {
	upper := strings.ToUpper(name)
	if explicitSecretNames[upper] {
		return true
	}
	for _, marker := range []string{"PASSWORD", "SECRET", "TOKEN", "KEY", "CREDENTIAL"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

func isCompilerLiftEnv(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasPrefix(upper, "MONOLIFT_LIFT_")
}

func envKeyForFlag(name string) string {
	return "FLAG_" + sanitizeKey(name)
}

func fileKey(source bootpath.FileSource) string {
	if source.MountName != "" {
		return sanitizeKey(source.MountName)
	}
	return sanitizeKey(source.Path)
}

func volumeName(source bootpath.FileSource) string {
	name := strings.ToLower(fileKey(source))
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "config"
	}
	return "boot-" + name
}

func sanitizeKey(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - ('a' - 'A')
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, value)
	value = strings.Trim(value, "_")
	if value == "" {
		return "CONFIG"
	}
	return value
}
