package codegen

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type LiftOptions struct {
	Source            string
	Target            string
	Trace             string
	Output            string
	ServiceName       string
	Deploy            DeployOptions
	WriteMonolithStub bool
}

type LiftResult struct {
	ActivationResult *activation.Result
	Report           reportv2.Report
	Cut              activation.CutResult
	Plan             *Plan
	Manifest         *Manifest
	PatchedFile      string
}

func RunLift(ctx context.Context, opts LiftOptions) error {
	_, err := RunLiftWithResult(ctx, opts)
	return err
}

func RunLiftWithResult(ctx context.Context, opts LiftOptions) (*LiftResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Source == "" {
		return nil, errors.New("codegen: source is required")
	}
	if opts.Target == "" {
		return nil, errors.New("codegen: target is required")
	}
	result, err := runActivation(ctx, opts)
	if err != nil {
		return nil, err
	}
	cut, err := activation.AnalyzeCut(result, nil)
	if err != nil {
		return nil, fmt.Errorf("analyze cut: %w", err)
	}
	report, err := buildExtractionReport(opts, cut)
	if err != nil {
		return nil, err
	}
	cutAdmission := AdmitCut(report, *cut)
	if !cutAdmission.Accepted {
		return nil, errors.New(cutAdmission.Error())
	}
	plan, err := BuildPlan(report, *cut)
	if err != nil {
		return nil, err
	}
	if err := attachIncomingCall(plan, result.Path, cut.Recommended.Step); err != nil {
		return nil, err
	}
	applyLiftOptions(plan, opts)
	plan.Admission = AdmitPlan(plan, cutAdmission)
	if !plan.Admission.Accepted {
		return nil, errors.New(plan.Admission.Error())
	}
	serverFiles, err := RenderServer(plan)
	if err != nil {
		return nil, err
	}
	clientFiles, err := RenderClient(plan)
	if err != nil {
		return nil, err
	}
	dockerFiles, err := RenderDockerfiles(plan)
	if err != nil {
		return nil, err
	}
	kubernetesFiles, err := RenderKubernetes(plan)
	if err != nil {
		return nil, err
	}
	artifacts := append(artifactsFromRendered("server", serverFiles), artifactsFromRendered("client_stub", clientFiles)...)
	artifacts = append(artifacts,
		Artifact{Path: plan.ExtractedDockerfilePath, Kind: "dockerfile_extracted", Content: dockerFiles[plan.ExtractedDockerfilePath]},
		Artifact{Path: plan.HostDockerfilePath, Kind: "dockerfile_host", Content: dockerFiles[plan.HostDockerfilePath]},
		Artifact{Path: plan.ExtractedDeploymentPath, Kind: "k8s_deployment_extracted", Content: kubernetesFiles[plan.ExtractedDeploymentPath]},
		Artifact{Path: plan.ExtractedServicePath, Kind: "k8s_service_extracted", Content: kubernetesFiles[plan.ExtractedServicePath]},
		Artifact{Path: plan.HostDeploymentPath, Kind: "k8s_deployment_host", Content: kubernetesFiles[plan.HostDeploymentPath]},
		Artifact{Path: plan.HostServicePath, Kind: "k8s_service_host", Content: kubernetesFiles[plan.HostServicePath]},
	)
	var patchedFile string
	var manifest *Manifest
	if opts.WriteMonolithStub {
		nonStubArtifacts := filterArtifacts(artifacts, "client_stub")
		entries, err := writeArtifactFiles(plan, nonStubArtifacts)
		if err != nil {
			return nil, err
		}
		stubContent := clientFiles[plan.ClientPath]
		patchedFile, err = PatchCutFunction(plan, stubContent)
		if err != nil {
			return nil, err
		}
		entries = append(entries, ManifestEntry{Path: plan.ClientPath, Kind: "client_stub"})
		manifest, err = writeManifest(plan, entries, patchedFile)
		if err != nil {
			return nil, err
		}
	} else {
		nonStubArtifacts := filterArtifacts(artifacts, "client_stub")
		manifest, err = WriteArtifacts(plan, nonStubArtifacts, patchedFile)
		if err != nil {
			return nil, err
		}
	}
	return &LiftResult{
		ActivationResult: result,
		Report:           report,
		Cut:              *cut,
		Plan:             plan,
		Manifest:         manifest,
		PatchedFile:      patchedFile,
	}, nil
}

func attachIncomingCall(plan *Plan, path *activation.Path, step int) error {
	if plan == nil {
		return errors.New("codegen: nil plan")
	}
	if path == nil || step <= 0 || step >= len(path.Steps) {
		return fmt.Errorf("codegen: missing incoming edge for cut step %d", step)
	}
	edge := path.Steps[step].Edge
	if edge == nil || edge.Position.File == "" || edge.Position.Line == 0 {
		return fmt.Errorf("codegen: incoming edge for cut step %d has no source position", step)
	}
	plan.Incoming = IncomingCall{
		File:   edge.Position.File,
		Line:   edge.Position.Line,
		Column: edge.Position.Column,
	}
	return nil
}

func runActivation(ctx context.Context, opts LiftOptions) (*activation.Result, error) {
	analyzer := activation.NewAnalyzer(activation.Config{
		Dir:           opts.Source,
		Packages:      []string{"./..."},
		Target:        opts.Target,
		Timeout:       10 * time.Minute,
		Augment:       activation.ModeAll,
		ScopePackages: true,
	})
	result, err := analyzer.Analyze(ctx)
	if err != nil {
		return result, fmt.Errorf("activation path: %w", err)
	}
	if result == nil || !result.Found || result.Path == nil {
		return result, fmt.Errorf("activation path not found for %s", opts.Target)
	}
	return result, nil
}

func buildExtractionReport(opts LiftOptions, cut *activation.CutResult) (reportv2.Report, error) {
	if cut == nil || cut.Recommended == nil {
		return reportv2.Report{}, errors.New("extract report: missing recommended cut")
	}
	file, line, err := activation.ParseTarget(opts.Target)
	if err != nil {
		return reportv2.Report{}, err
	}
	absFile, err := filepath.Abs(file)
	if err != nil {
		return reportv2.Report{}, err
	}
	name := opts.ServiceName
	if name == "" {
		name = "monolift-" + cut.Recommended.NodeKey.FuncName
	}
	pragma := &compiler.Pragma{
		Name:    sanitizeServiceName(name),
		Surface: compiler.SurfaceFunction,
		Options: map[string]string{
			"name":      sanitizeServiceName(name),
			"mode":      "remote",
			"transport": "httpjson",
		},
		Span: compiler.Span{
			Filename: absFile,
			Line:     line,
			EndLine:  line,
		},
		DeclName:     cut.Recommended.NodeKey.FuncName,
		DeclKind:     "func",
		DeclIdentity: cut.Recommended.NodeKey.String(),
	}
	report, _, err := compiler.Extract([]string{opts.Source}, []*compiler.Pragma{pragma})
	if err != nil {
		return reportv2.Report{}, fmt.Errorf("extract report: %w", err)
	}
	if report.BuildConfig.ModuleRoot == "" {
		report.BuildConfig.ModuleRoot = opts.Source
	}
	return report, nil
}

func applyLiftOptions(plan *Plan, opts LiftOptions) {
	if opts.ServiceName != "" {
		plan.ServiceName = sanitizeServiceName(opts.ServiceName)
		plan.EnvServiceName = envServiceName(plan.ServiceName)
	}
	if opts.Output != "" {
		output := opts.Output
		if !filepath.IsAbs(output) {
			output = filepath.Join(plan.SourceModuleRoot, output)
		}
		plan.OutputDir = output
	}
	plan.ServerPath = filepath.Join(plan.OutputDir, "cmd", plan.ServiceName, "main.go")
	plan.ClientPath = filepath.Join(plan.CutPoint.PackageDir, "monolift_lift_"+plan.EnvServiceName+".go")
	plan.ManifestPath = filepath.Join(plan.OutputDir, ManifestName)
	applyDeployDefaults(plan, opts.Deploy)
}

func applyDeployDefaults(plan *Plan, opts DeployOptions) {
	deploy := opts
	if deploy.HostServiceName == "" {
		deploy.HostServiceName = sanitizeServiceName(plan.ServiceName + "-host")
	} else {
		deploy.HostServiceName = sanitizeServiceName(deploy.HostServiceName)
	}
	if deploy.ExtractedServiceName == "" {
		deploy.ExtractedServiceName = sanitizeServiceName(plan.ServiceName)
	} else {
		deploy.ExtractedServiceName = sanitizeServiceName(deploy.ExtractedServiceName)
	}
	if deploy.HostImage == "" {
		deploy.HostImage = deploy.HostServiceName + ":latest"
	}
	if deploy.ExtractedImage == "" {
		deploy.ExtractedImage = deploy.ExtractedServiceName + ":latest"
	}
	if deploy.HostPort == 0 {
		deploy.HostPort = 8080
	}
	if deploy.ExtractedPort == 0 {
		deploy.ExtractedPort = 8081
	}
	if deploy.HostReadinessPath == "" {
		deploy.HostReadinessPath = "/healthz"
	}
	if deploy.HostBuildPackage == "" {
		deploy.HostBuildPackage = "."
	}
	if deploy.HostBinaryName == "" {
		deploy.HostBinaryName = deploy.HostServiceName
	}
	if deploy.HostRuntimeImage == "" {
		deploy.HostRuntimeImage = "gcr.io/distroless/static-debian12"
	}
	if deploy.ImagePullPolicy == "" {
		deploy.ImagePullPolicy = "IfNotPresent"
	}
	plan.Deploy = deploy

	manifestDir := filepath.Join(plan.OutputDir, "manifests")
	plan.HostDockerfilePath = filepath.Join(plan.OutputDir, "Dockerfile.host-"+plan.Deploy.HostServiceName)
	plan.ExtractedDockerfilePath = filepath.Join(plan.OutputDir, "Dockerfile.extracted-"+plan.Deploy.ExtractedServiceName)
	plan.HostDeploymentPath = filepath.Join(manifestDir, plan.Deploy.HostServiceName+"-deployment.yaml")
	plan.HostServicePath = filepath.Join(manifestDir, plan.Deploy.HostServiceName+"-service.yaml")
	plan.ExtractedDeploymentPath = filepath.Join(manifestDir, plan.Deploy.ExtractedServiceName+"-deployment.yaml")
	plan.ExtractedServicePath = filepath.Join(manifestDir, plan.Deploy.ExtractedServiceName+"-service.yaml")
}
