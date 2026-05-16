package codegen

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

func RenderKubernetes(plan *Plan) (map[string][]byte, error) {
	if plan == nil {
		return nil, fmt.Errorf("codegen: nil plan")
	}
	view := kubernetesView{
		Plan:                  plan,
		HostEnvVars:           hostDeploymentEnvVars(plan),
		ExtractedEnvVars:      extractedDeploymentEnvVars(plan),
		HostVolumeMounts:      effectiveHostVolumeMounts(plan),
		ExtractedVolumeMounts: effectiveExtractedVolumeMounts(plan),
		SharedVolumeMounts:    effectiveSharedVolumeMounts(plan),
	}
	files := map[string][]byte{}
	if len(view.SharedVolumeMounts) > 0 {
		if plan.SharedVolumeClaimPath == "" {
			return nil, fmt.Errorf("codegen: shared volume mounts require SharedVolumeClaimPath")
		}
		if err := renderYAML(files, plan.SharedVolumeClaimPath, sharedVolumeClaimTemplate, view); err != nil {
			return nil, err
		}
	}
	if err := renderYAML(files, plan.ExtractedDeploymentPath, extractedDeploymentTemplate, view); err != nil {
		return nil, err
	}
	if err := renderYAML(files, plan.ExtractedServicePath, extractedServiceTemplate, view); err != nil {
		return nil, err
	}
	if err := renderYAML(files, plan.HostDeploymentPath, hostDeploymentTemplate, view); err != nil {
		return nil, err
	}
	if err := renderYAML(files, plan.HostServicePath, hostServiceTemplate, view); err != nil {
		return nil, err
	}
	return files, nil
}

type kubernetesView struct {
	Plan                  *Plan
	HostEnvVars           []EnvVar
	ExtractedEnvVars      []EnvVar
	HostVolumeMounts      []VolumeMount
	ExtractedVolumeMounts []VolumeMount
	SharedVolumeMounts    []SharedVolumeMount
}

func renderYAML(files map[string][]byte, path, source string, view kubernetesView) error {
	tmpl, err := template.New("kubernetes").Funcs(template.FuncMap{
		"yamlQuote": strconv.Quote,
	}).Parse(source)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, view); err != nil {
		return err
	}
	files[path] = out.Bytes()
	return nil
}

func hostDeploymentEnvVars(plan *Plan) []EnvVar {
	if plan == nil {
		return nil
	}
	envName := plan.EnvServiceName
	env := []EnvVar{
		{Name: "MONOLIFT_LIFT_" + envName, Value: "on"},
		{Name: "MONOLIFT_LIFT_FAILMODE", Value: "closed"},
		{Name: "MONOLIFT_" + envName + "_ENDPOINT", Value: fmt.Sprintf("http://%s:%d/invoke", plan.Deploy.ExtractedServiceName, plan.Deploy.ExtractedPort)},
	}
	env = append(env, plan.Deploy.HostEnvVars...)
	return env
}

func extractedDeploymentEnvVars(plan *Plan) []EnvVar {
	if plan == nil {
		return nil
	}
	env := append([]EnvVar(nil), plan.Deploy.ExtractedEnvVars...)
	env = appendMissingEnvVars(env, reconstructorExtractedEnvVars(plan)...)
	if !planHasSQLReconstructor(plan) || hasEnvVar(env, "DATABASE_URL") {
		return env
	}
	if databaseURL, ok := findEnvVar(plan.Deploy.HostEnvVars, "DATABASE_URL"); ok {
		env = append(env, databaseURL)
	}
	return env
}

func appendMissingEnvVars(env []EnvVar, additions ...EnvVar) []EnvVar {
	for _, item := range additions {
		if item.Name == "" || hasEnvVar(env, item.Name) {
			continue
		}
		env = append(env, item)
	}
	return env
}

func reconstructorExtractedEnvVars(plan *Plan) []EnvVar {
	var env []EnvVar
	for _, reconstructor := range planReconstructors(plan) {
		env = append(env, reconstructor.ExtractedEnvVars...)
	}
	return env
}

func effectiveHostVolumeMounts(plan *Plan) []VolumeMount {
	if plan == nil {
		return nil
	}
	mounts := append([]VolumeMount(nil), plan.Deploy.HostVolumeMounts...)
	for _, shared := range effectiveSharedVolumeMounts(plan) {
		mounts = appendMissingVolumeMount(mounts, VolumeMount{Name: shared.Name, MountPath: shared.MountPath})
	}
	return mounts
}

func effectiveExtractedVolumeMounts(plan *Plan) []VolumeMount {
	if plan == nil {
		return nil
	}
	mounts := append([]VolumeMount(nil), plan.Deploy.ExtractedVolumeMounts...)
	for _, shared := range effectiveSharedVolumeMounts(plan) {
		mounts = appendMissingVolumeMount(mounts, VolumeMount{Name: shared.Name, MountPath: shared.MountPath})
	}
	return mounts
}

func appendMissingVolumeMount(mounts []VolumeMount, addition VolumeMount) []VolumeMount {
	if addition.Name == "" || addition.MountPath == "" {
		return mounts
	}
	for _, item := range mounts {
		if item.Name == addition.Name || item.MountPath == addition.MountPath {
			return mounts
		}
	}
	return append(mounts, addition)
}

func effectiveSharedVolumeMounts(plan *Plan) []SharedVolumeMount {
	if plan == nil {
		return nil
	}
	mounts := append([]SharedVolumeMount(nil), plan.Deploy.SharedVolumeMounts...)
	for _, reconstructor := range planReconstructors(plan) {
		for _, mount := range reconstructor.SharedVolumeMounts {
			mounts = appendMissingSharedVolumeMount(mounts, renderSharedVolumeMount(plan, mount))
		}
	}
	return mounts
}

func appendMissingSharedVolumeMount(mounts []SharedVolumeMount, addition SharedVolumeMount) []SharedVolumeMount {
	if addition.Name == "" || addition.ClaimName == "" || addition.MountPath == "" {
		return mounts
	}
	for _, item := range mounts {
		if item.Name == addition.Name || item.ClaimName == addition.ClaimName || item.MountPath == addition.MountPath {
			return mounts
		}
	}
	return append(mounts, addition)
}

func renderSharedVolumeMount(plan *Plan, mount SharedVolumeMount) SharedVolumeMount {
	serviceName := ""
	if plan != nil {
		serviceName = plan.Deploy.ExtractedServiceName
		if serviceName == "" {
			serviceName = plan.ServiceName
		}
	}
	mount.ClaimName = strings.ReplaceAll(mount.ClaimName, "${SERVICE}", serviceName)
	if mount.StorageRequest == "" {
		mount.StorageRequest = "1Gi"
	}
	return mount
}

func planHasSQLReconstructor(plan *Plan) bool {
	if plan == nil {
		return false
	}
	for _, param := range plan.ReconstructedParams {
		switch param.Reconstructor.ID {
		case "sql_db", "sql_db_wrapper":
			return true
		}
	}
	return false
}

func hasEnvVar(env []EnvVar, name string) bool {
	_, ok := findEnvVar(env, name)
	return ok
}

func findEnvVar(env []EnvVar, name string) (EnvVar, bool) {
	for _, item := range env {
		if item.Name == name {
			return item, true
		}
	}
	return EnvVar{}, false
}

const extractedDeploymentTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Plan.Deploy.ExtractedServiceName }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .Plan.Deploy.ExtractedServiceName }}
  template:
    metadata:
      labels:
        app: {{ .Plan.Deploy.ExtractedServiceName }}
    spec:
      containers:
        - name: extracted
          image: {{ .Plan.Deploy.ExtractedImage }}
          imagePullPolicy: {{ .Plan.Deploy.ImagePullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Plan.Deploy.ExtractedPort }}
{{- if .ExtractedEnvVars }}
          env:
{{- range .ExtractedEnvVars }}
            - name: {{ .Name }}
              value: {{ yamlQuote .Value }}
{{- end }}
{{- end }}
{{- if .ExtractedVolumeMounts }}
          volumeMounts:
{{- range .ExtractedVolumeMounts }}
            - name: {{ .Name }}
              mountPath: {{ .MountPath }}
{{- end }}
{{- end }}
          readinessProbe:
            httpGet:
              path: /healthz
              port: {{ .Plan.Deploy.ExtractedPort }}
            periodSeconds: 2
{{- if .SharedVolumeMounts }}
      volumes:
{{- range .SharedVolumeMounts }}
        - name: {{ .Name }}
          persistentVolumeClaim:
            claimName: {{ .ClaimName }}
{{- end }}
{{- end }}
`

const extractedServiceTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Plan.Deploy.ExtractedServiceName }}
spec:
  type: ClusterIP
  selector:
    app: {{ .Plan.Deploy.ExtractedServiceName }}
  ports:
    - name: http
      port: {{ .Plan.Deploy.ExtractedPort }}
      targetPort: {{ .Plan.Deploy.ExtractedPort }}
`

const hostDeploymentTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Plan.Deploy.HostServiceName }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .Plan.Deploy.HostServiceName }}
  template:
    metadata:
      labels:
        app: {{ .Plan.Deploy.HostServiceName }}
    spec:
{{- if .Plan.Deploy.HostRunAsUser }}
      securityContext:
        runAsUser: {{ .Plan.Deploy.HostRunAsUser }}
        runAsGroup: {{ .Plan.Deploy.HostRunAsUser }}
{{- end }}
      containers:
        - name: host
          image: {{ .Plan.Deploy.HostImage }}
          imagePullPolicy: {{ .Plan.Deploy.ImagePullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Plan.Deploy.HostPort }}
{{- if .Plan.Deploy.HostArgs }}
          command:
{{- range .Plan.Deploy.HostArgs }}
            - {{ yamlQuote . }}
{{- end }}
{{- end }}
          readinessProbe:
            httpGet:
              path: {{ .Plan.Deploy.HostReadinessPath }}
              port: {{ .Plan.Deploy.HostPort }}
            periodSeconds: 2
          env:
{{- range .HostEnvVars }}
            - name: {{ .Name }}
              value: {{ yamlQuote .Value }}
{{- end }}
{{- if .HostVolumeMounts }}
          volumeMounts:
{{- range .HostVolumeMounts }}
            - name: {{ .Name }}
              mountPath: {{ .MountPath }}
{{- end }}
{{- end }}
{{- if or .Plan.Deploy.HostConfigMapVolumes .Plan.Deploy.HostEmptyDirVolumes .SharedVolumeMounts }}
      volumes:
{{- range .Plan.Deploy.HostConfigMapVolumes }}
        - name: {{ .Name }}
          configMap:
            name: {{ .ConfigMapName }}
{{- end }}
{{- range .Plan.Deploy.HostEmptyDirVolumes }}
        - name: {{ . }}
          emptyDir: {}
{{- end }}
{{- range .SharedVolumeMounts }}
        - name: {{ .Name }}
          persistentVolumeClaim:
            claimName: {{ .ClaimName }}
{{- end }}
{{- end }}
`

const hostServiceTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Plan.Deploy.HostServiceName }}
spec:
  type: ClusterIP
  selector:
    app: {{ .Plan.Deploy.HostServiceName }}
  ports:
    - name: http
      port: {{ .Plan.Deploy.HostPort }}
      targetPort: {{ .Plan.Deploy.HostPort }}
`

const sharedVolumeClaimTemplate = `{{- range .SharedVolumeMounts }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ .ClaimName }}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ .StorageRequest }}
---
{{- end }}
`
