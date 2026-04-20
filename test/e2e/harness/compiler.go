package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type Compiler struct {
	Path      string
	OutputDir string
}

type CompileResult struct {
	ArtifactsDir string
	Report       *reportv2.Report
	RawStderr    string
	RawStdout    string
	ExitCode     int
}

func (c Compiler) Run(ctx context.Context, target TargetCase) (CompileResult, error) {
	outputDir := c.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(os.TempDir(), "monolift-e2e", target.Name, fmt.Sprintf("%d", time.Now().UnixNano()), "compile")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return CompileResult{ArtifactsDir: outputDir, ExitCode: -1}, err
	}

	path := c.Path
	if path == "" {
		path = CompilerPath()
	}
	result, err := RunCommand(ctx, path, "--target="+target.Name, "--output="+outputDir)
	compileResult := CompileResult{
		ArtifactsDir: outputDir,
		RawStderr:    result.Stderr,
		RawStdout:    result.Stdout,
		ExitCode:     result.ExitCode,
	}
	if err != nil {
		return compileResult, err
	}

	reportData, err := os.ReadFile(filepath.Join(outputDir, "closure-report.json"))
	if err != nil {
		return compileResult, err
	}
	report, err := reportv2.Parse(reportData)
	if err != nil {
		return compileResult, err
	}
	compileResult.Report = report
	return compileResult, nil
}

func FormatCompileFailure(target TargetCase, result CompileResult, err error) error {
	got := "missing"
	if result.Report != nil {
		got = result.Report.Pragma.Options["verdict"]
	}
	return StageError(
		3,
		target.Name,
		KindCompiler,
		"compile exit=%d verdict=got_%s want_%s stderr: %s: %v",
		result.ExitCode,
		got,
		target.ExpectedVerdict,
		TailLines(result.RawStderr, 20),
		err,
	)
}
