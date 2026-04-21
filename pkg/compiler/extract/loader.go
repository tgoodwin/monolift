package extract

import (
	"fmt"
	"go/build"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

type loadedModule struct {
	RootPragma Pragma
	RootFile   string
	ModuleRoot string
	GOOS       string
	GOARCH     string
	CGOEnabled bool
	BuildTags  []string
	Fset       *token.FileSet
	Packages   []*packages.Package
	RootPkg    *packages.Package
}

func loadModule(req Request) (*loadedModule, error) {
	rootPragma, err := selectRootPragma(req.Pragmas)
	if err != nil {
		return nil, err
	}

	rootFile, err := filepath.Abs(rootPragma.Span.Filename)
	if err != nil {
		return nil, fmt.Errorf("resolve root file %q: %w", rootPragma.Span.Filename, err)
	}
	moduleRoot, err := findModuleRoot(rootFile)
	if err != nil {
		return nil, err
	}
	toolchain := moduleToolchain(moduleRoot)
	goos := envOrDefault("GOOS", runtime.GOOS)
	goarch := envOrDefault("GOARCH", runtime.GOARCH)
	cgoEnabled := envOrDefault("CGO_ENABLED", boolToEnv(build.Default.CgoEnabled))
	buildTags := parseBuildTags(os.Getenv("GOFLAGS"))

	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode:       packages.LoadAllSyntax | packages.NeedModule,
		Dir:        moduleRoot,
		Fset:       fset,
		Tests:      false,
		Env:        loaderEnv(goos, goarch, cgoEnabled, toolchain),
		BuildFlags: loaderBuildFlags(buildTags),
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("packages.Load from %s: %w", moduleRoot, err)
	}
	if errs := collectPackageErrors(pkgs); len(errs) > 0 {
		return nil, fmt.Errorf("packages.Load reported errors for %s:\n%s", moduleRoot, strings.Join(errs, "\n"))
	}

	rootPkg := findPackageForFile(pkgs, rootFile)
	if rootPkg == nil {
		return nil, fmt.Errorf("root file %s was not present in loaded package graph rooted at %s", rootFile, moduleRoot)
	}

	return &loadedModule{
		RootPragma: rootPragma,
		RootFile:   rootFile,
		ModuleRoot: moduleRoot,
		GOOS:       goos,
		GOARCH:     goarch,
		CGOEnabled: cgoEnabled != "0",
		BuildTags:  buildTags,
		Fset:       fset,
		Packages:   pkgs,
		RootPkg:    rootPkg,
	}, nil
}

func selectRootPragma(pragmas []Pragma) (Pragma, error) {
	var roots []Pragma
	for _, pragma := range pragmas {
		if pragma.Span.Filename == "" {
			continue
		}
		roots = append(roots, pragma)
	}
	switch len(roots) {
	case 0:
		return Pragma{}, fmt.Errorf("no parsed monolift root pragma supplied to extract.Analyze")
	case 1:
		return roots[0], nil
	default:
		return Pragma{}, fmt.Errorf("extract.Analyze currently supports one root pragma, got %d", len(roots))
	}
}

func findModuleRoot(rootFile string) (string, error) {
	dir := filepath.Dir(rootFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above %s", rootFile)
		}
		dir = parent
	}
}

func collectPackageErrors(pkgs []*packages.Package) []string {
	var out []string
	for _, pkg := range pkgs {
		for _, pkgErr := range pkg.Errors {
			out = append(out, fmt.Sprintf("%s: %s", pkg.PkgPath, pkgErr.Msg))
		}
	}
	return out
}

func findPackageForFile(pkgs []*packages.Package, filename string) *packages.Package {
	for _, pkg := range pkgs {
		for _, compiled := range pkg.CompiledGoFiles {
			if samePath(compiled, filename) {
				return pkg
			}
		}
		for _, source := range pkg.GoFiles {
			if samePath(source, filename) {
				return pkg
			}
		}
	}
	return nil
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func loaderEnv(goos, goarch, cgoEnabled, toolchain string) []string {
	env := append([]string{}, os.Environ()...)
	env = upsertEnv(env, "GOOS", goos)
	env = upsertEnv(env, "GOARCH", goarch)
	env = upsertEnv(env, "CGO_ENABLED", cgoEnabled)
	env = upsertEnv(env, "GOTOOLCHAIN", envOrDefault("GOTOOLCHAIN", toolchain))
	return env
}

func loaderBuildFlags(buildTags []string) []string {
	if len(buildTags) == 0 {
		return nil
	}
	return []string{"-tags=" + strings.Join(buildTags, ",")}
}

func parseBuildTags(goFlags string) []string {
	if goFlags == "" {
		return nil
	}

	var tags []string
	fields := strings.Fields(goFlags)
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		switch {
		case strings.HasPrefix(field, "-tags="):
			tags = append(tags, splitBuildTagList(strings.TrimPrefix(field, "-tags="))...)
		case field == "-tags" && i+1 < len(fields):
			i++
			tags = append(tags, splitBuildTagList(fields[i])...)
		}
	}
	if len(tags) == 0 {
		return nil
	}
	slices.Sort(tags)
	return slices.Compact(tags)
}

func splitBuildTagList(value string) []string {
	value = strings.ReplaceAll(value, ",", " ")
	parts := strings.Fields(value)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, `"'`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func upsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func boolToEnv(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func moduleToolchain(moduleRoot string) string {
	goModPath := filepath.Join(moduleRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "auto"
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "toolchain ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "toolchain "))
		}
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "go "))
			if version == "" {
				break
			}
			if !strings.HasPrefix(version, "go") {
				version = "go" + version
			}
			return version
		}
	}
	return "auto"
}
