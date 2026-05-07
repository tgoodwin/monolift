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
	WriteMonolithStub bool
}

func RunLift(ctx context.Context, opts LiftOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Source == "" {
		return errors.New("codegen: source is required")
	}
	if opts.Target == "" {
		return errors.New("codegen: target is required")
	}
	result, err := runActivation(ctx, opts)
	if err != nil {
		return err
	}
	cut, err := activation.AnalyzeCut(result, nil)
	if err != nil {
		return fmt.Errorf("analyze cut: %w", err)
	}
	report, err := buildExtractionReport(opts, cut)
	if err != nil {
		return err
	}
	cutAdmission := AdmitCut(report, *cut)
	if !cutAdmission.Accepted {
		return errors.New(cutAdmission.Error())
	}
	plan, err := BuildPlan(report, *cut)
	if err != nil {
		return err
	}
	if err := attachIncomingCall(plan, result.Path, cut.Recommended.Step); err != nil {
		return err
	}
	applyLiftOptions(plan, opts)
	plan.Admission = AdmitPlan(plan, cutAdmission)
	if !plan.Admission.Accepted {
		return errors.New(plan.Admission.Error())
	}
	serverFiles, err := RenderServer(plan)
	if err != nil {
		return err
	}
	clientFiles, err := RenderClient(plan)
	if err != nil {
		return err
	}
	var patchedFile string
	artifacts := append(artifactsFromRendered("server", serverFiles), artifactsFromRendered("client_stub", clientFiles)...)
	if opts.WriteMonolithStub {
		entries, err := writeArtifactFiles(plan, artifacts)
		if err != nil {
			return err
		}
		patchedFile, err = PatchCallsite(plan)
		if err != nil {
			return err
		}
		if _, err := writeManifest(plan, entries, patchedFile); err != nil {
			return err
		}
		return nil
	}
	if _, err := WriteArtifacts(plan, artifacts, patchedFile); err != nil {
		return err
	}
	return nil
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
		Dir:      opts.Source,
		Packages: []string{"./..."},
		Target:   opts.Target,
		Timeout:  120 * time.Second,
		Augment:  activation.ModeAll,
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
}
