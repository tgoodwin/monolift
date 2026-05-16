package codegen

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
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
	Augment           activation.AugmentMode
	Deploy            DeployOptions
	WriteMonolithStub bool
}

type LiftResult struct {
	ActivationResult *activation.Result
	Timings          []activation.PhaseTiming
	Report           reportv2.Report
	Cut              activation.CutResult
	AdmissionVerdict AdmissionVerdict
	DemotionChain    []CandidateDemotion
	Plan             *Plan
	Manifest         *Manifest
	PatchedFile      string
}

type CandidateDemotion struct {
	Step        int                    `json:"step"`
	NodeKey     activation.FunctionKey `json:"node_key"`
	NodeName    string                 `json:"node_name,omitempty"`
	RefusalCode string                 `json:"refusal_code"`
	Message     string                 `json:"message,omitempty"`
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
	var timings []activation.PhaseTiming
	progress := newLiftProgressLogger(opts.Target)
	result, err := timeLiftPhase(&timings, progress, "activation", func() (*activation.Result, error) {
		return runActivation(ctx, opts)
	})
	if err != nil {
		return nil, err
	}
	cut, err := timeLiftPhase(&timings, progress, "cut", func() (*activation.CutResult, error) {
		return activation.AnalyzeCut(result, nil)
	})
	if err != nil {
		return nil, fmt.Errorf("analyze cut: %w", err)
	}
	report, err := timeLiftPhase(&timings, progress, "extract-report", func() (reportv2.Report, error) {
		return buildExtractionReport(opts, cut)
	})
	if err != nil {
		return nil, err
	}
	var demotionChain []CandidateDemotion
	admissionPhase := "admit-cut"
	if admissionAwareRankEnabled() {
		admissionPhase = "admit-candidate"
	}
	cutAdmission, err := timeLiftPhase(&timings, progress, admissionPhase, func() (AdmissionVerdict, error) {
		if !admissionAwareRankEnabled() {
			return AdmitCut(report, *cut), nil
		}
		verdict, chain, err := admitCutCandidates(report, cut)
		demotionChain = chain
		return verdict, err
	})
	if err != nil {
		return nil, err
	}
	if !cutAdmission.Accepted {
		return &LiftResult{
			ActivationResult: result,
			Timings:          timings,
			Report:           report,
			Cut:              *cut,
			AdmissionVerdict: cutAdmission,
			DemotionChain:    demotionChain,
		}, errors.New(cutAdmission.Error())
	}
	plan, err := timeLiftPhase(&timings, progress, "build-plan", func() (*Plan, error) {
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
		return plan, nil
	})
	if err != nil {
		return nil, err
	}
	serverFiles, err := timeLiftPhase(&timings, progress, "render-server", func() (map[string][]byte, error) {
		return RenderServer(plan)
	})
	if err != nil {
		return nil, err
	}
	clientFiles, err := timeLiftPhase(&timings, progress, "render-client", func() (map[string][]byte, error) {
		return RenderClient(plan)
	})
	if err != nil {
		return nil, err
	}
	dockerFiles, err := timeLiftPhase(&timings, progress, "render-dockerfiles", func() (map[string][]byte, error) {
		return RenderDockerfiles(plan)
	})
	if err != nil {
		return nil, err
	}
	kubernetesFiles, err := timeLiftPhase(&timings, progress, "render-kubernetes", func() (map[string][]byte, error) {
		return RenderKubernetes(plan)
	})
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
	if plan.SharedVolumeClaimPath != "" {
		artifacts = append(artifacts, Artifact{Path: plan.SharedVolumeClaimPath, Kind: "k8s_persistent_volume_claim", Content: kubernetesFiles[plan.SharedVolumeClaimPath]})
	}
	var patchedFile string
	var manifest *Manifest
	if opts.WriteMonolithStub {
		nonStubArtifacts := filterArtifacts(artifacts, "client_stub")
		entries, err := timeLiftPhase(&timings, progress, "write-artifacts", func() ([]ManifestEntry, error) {
			return writeArtifactFiles(plan, nonStubArtifacts)
		})
		if err != nil {
			return nil, err
		}
		stubContent := clientFiles[plan.ClientPath]
		patchedFile, err = timeLiftPhase(&timings, progress, "patch-function", func() (string, error) {
			return PatchCutFunctionProfile(plan, stubContent, &timings)
		})
		if err != nil {
			return nil, err
		}
		entries = append(entries, ManifestEntry{Path: plan.ClientPath, Kind: "client_stub"})

		// Render and write the same-package invocation adapter.
		adapterFiles, err := timeLiftPhase(&timings, progress, "render-adapter", func() (map[string][]byte, error) {
			return RenderAdapter(plan)
		})
		if err != nil {
			return nil, err
		}
		for adapterPath, adapterContent := range adapterFiles {
			if err := writeAtomic(adapterPath, withGeneratedHeader(plan, adapterContent), 0644); err != nil {
				return nil, err
			}
			entries = append(entries, ManifestEntry{Path: adapterPath, Kind: "adapter"})
		}

		manifest, err = writeManifest(plan, entries, patchedFile)
		if err != nil {
			return nil, err
		}
	} else {
		nonStubArtifacts := filterArtifacts(artifacts, "client_stub")
		manifest, err = timeLiftPhase(&timings, progress, "write-artifacts", func() (*Manifest, error) {
			return WriteArtifacts(plan, nonStubArtifacts, patchedFile)
		})
		if err != nil {
			return nil, err
		}
	}
	return &LiftResult{
		ActivationResult: result,
		Timings:          timings,
		Report:           report,
		Cut:              *cut,
		AdmissionVerdict: plan.Admission,
		DemotionChain:    demotionChain,
		Plan:             plan,
		Manifest:         manifest,
		PatchedFile:      patchedFile,
	}, nil
}

type liftProgressLogger struct {
	logger  *slog.Logger
	started time.Time
}

func newLiftProgressLogger(target string) *liftProgressLogger {
	return &liftProgressLogger{
		logger:  slog.Default().With("component", "codegen", "target", target),
		started: time.Now(),
	}
}

func (l *liftProgressLogger) start(phase string) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Debug("codegen phase", "phase", phase, "event", "start", "elapsed", time.Since(l.started).Round(time.Second))
}

func (l *liftProgressLogger) done(phase string, started time.Time, err error) {
	if l == nil || l.logger == nil {
		return
	}
	status := "done"
	if err != nil {
		status = "error"
	}
	l.logger.Debug("codegen phase", "phase", phase, "event", status, "duration", time.Since(started).Round(time.Millisecond), "elapsed", time.Since(l.started).Round(time.Second))
}

func timeLiftPhase[T any](timings *[]activation.PhaseTiming, progress *liftProgressLogger, phase string, fn func() (T, error)) (T, error) {
	start := time.Now()
	progress.start(phase)
	value, err := fn()
	progress.done(phase, start, err)
	if timings != nil {
		*timings = append(*timings, activation.PhaseTiming{Phase: phase, Duration: time.Since(start)})
	}
	return value, err
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
	augment := opts.Augment
	if augment == "" {
		augment = activation.ModeAll
	}
	// ScopePackages fills Packages from the target's reverse-import set before
	// type-checking; keep the fallback narrow rather than whole-repository ./...
	analyzer := activation.NewAnalyzer(activation.Config{
		Dir:           opts.Source,
		Packages:      nil,
		Target:        opts.Target,
		Timeout:       10 * time.Minute,
		Augment:       augment,
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
	absFile, err := targetFileForReport(opts.Source, file)
	if err != nil {
		return reportv2.Report{}, err
	}
	name := opts.ServiceName
	if name == "" {
		name = "monolift-" + cut.Recommended.NodeKey.FuncName
	}
	surface := compiler.SurfaceFunction
	declName := cut.Recommended.NodeKey.FuncName
	declKind := "func"
	if cut.Recommended.NodeKey.Receiver != "" {
		surface = compiler.SurfaceMethod
		declKind = "method"
		recv := cut.Recommended.NodeKey.Receiver
		if strings.HasPrefix(recv, "*") {
			declName = "(*" + recv[1:] + ")." + declName
		} else {
			declName = recv + "." + declName
		}
	}
	pragma := &compiler.Pragma{
		Name:    sanitizeServiceName(name),
		Surface: surface,
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
		DeclName:     declName,
		DeclKind:     declKind,
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

func targetFileForReport(source, file string) (string, error) {
	if !filepath.IsAbs(file) {
		file = filepath.Join(source, file)
	}
	return filepath.Abs(file)
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
	if len(effectiveSharedVolumeMounts(plan)) > 0 {
		plan.SharedVolumeClaimPath = filepath.Join(manifestDir, plan.Deploy.ExtractedServiceName+"-shared-volumes.yaml")
	}
}
