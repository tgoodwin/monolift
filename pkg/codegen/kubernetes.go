package codegen

import (
	"bytes"
	"fmt"
	"strconv"
	"text/template"
)

func RenderKubernetes(plan *Plan) (map[string][]byte, error) {
	if plan == nil {
		return nil, fmt.Errorf("codegen: nil plan")
	}
	view := kubernetesView{
		Plan:        plan,
		HostEnvVars: hostDeploymentEnvVars(plan),
	}
	files := map[string][]byte{}
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
	Plan        *Plan
	HostEnvVars []EnvVar
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
          readinessProbe:
            httpGet:
              path: /healthz
              port: {{ .Plan.Deploy.ExtractedPort }}
            periodSeconds: 2
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
{{- if .Plan.Deploy.HostVolumeMounts }}
          volumeMounts:
{{- range .Plan.Deploy.HostVolumeMounts }}
            - name: {{ .Name }}
              mountPath: {{ .MountPath }}
{{- end }}
{{- end }}
{{- if or .Plan.Deploy.HostConfigMapVolumes .Plan.Deploy.HostEmptyDirVolumes }}
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
