package codegen

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/packages"
)

func BuildPlan(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
	if cut.Recommended == nil {
		return nil, errors.New("codegen: cut has no recommended candidate")
	}
	moduleRoot, err := sourceModuleRoot(report)
	if err != nil {
		return nil, err
	}
	pkgPath := cut.Recommended.NodeKey.PackagePath
	if pkgPath == "" {
		pkgPath = report.Root.Identity.PackagePath
	}
	funcName := cut.Recommended.NodeKey.FuncName
	if funcName == "" {
		funcName = report.Root.Identity.ObjectName
	}
	if pkgPath == "" || funcName == "" {
		return nil, fmt.Errorf("codegen: missing cut identity package=%q func=%q", pkgPath, funcName)
	}

	pkg, err := loadPackage(moduleRoot, pkgPath)
	if err != nil {
		return nil, err
	}

	receiver := cut.Recommended.NodeKey.Receiver
	var sig *types.Signature

	// Receiver policy selection: when a receiver is present, look up the
	// method on the named type instead of the package scope.
	var receiverParam *ReceiverSpec
	if receiver != "" {
		named, isPointer, err := lookupReceiverType(pkg.Types, receiver)
		if err != nil {
			return nil, err
		}
		_, methodSig, err := lookupMethod(named, isPointer, funcName)
		if err != nil {
			return nil, err
		}
		sig = methodSig

		stateClass := cut.Recommended.State
		spec, err := selectReceiverPolicy(named, isPointer, stateClass)
		if err != nil {
			return nil, err
		}
		receiverParam = spec
	} else {
		fn, ok := pkg.Types.Scope().Lookup(funcName).(*types.Func)
		if !ok || fn == nil {
			return nil, fmt.Errorf("codegen: function %s not found in %s", funcName, pkgPath)
		}
		fnSig, ok := fn.Type().(*types.Signature)
		if !ok || fnSig == nil {
			return nil, fmt.Errorf("codegen: %s is not a function signature", funcName)
		}
		sig = fnSig
	}

	position := findFunctionPosition(pkg, funcName, receiver)

	plan := &Plan{
		SourceModuleRoot: moduleRoot,
		SourceModulePath: modulePath(report, pkg),
		ServiceName:      serviceName(report, funcName),
		ReceiverParam:    receiverParam,
		CutPoint: CutPoint{
			PackagePath: pkgPath,
			PackageName: pkg.Name,
			PackageDir:  packageDir(pkg),
			FuncName:    funcName,
			Receiver:    receiver,
			File:        position.Filename,
			Line:        position.Line,
			Column:      position.Column,
			Key:         cut.Recommended.NodeKey,
		},
		Admission: AdmissionVerdict{
			Accepted: true,
			Reasons:  []string{"plan built for recommended cut"},
			Cut:      cut.Recommended,
		},
	}
	plan.EnvServiceName = envServiceName(plan.ServiceName)
	plan.OutputDir = filepath.Join(moduleRoot, "monolift_gen", plan.ServiceName)
	plan.ServerPath = filepath.Join(plan.OutputDir, "cmd", plan.ServiceName, "main.go")
	plan.ClientPath = filepath.Join(plan.CutPoint.PackageDir, "monolift_lift_"+plan.EnvServiceName+".go")
	plan.ManifestPath = filepath.Join(plan.OutputDir, ManifestName)

	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		param, err := MapParam(funcName, i, params.At(i), pkgPath)
		if err != nil {
			return nil, err
		}
		// Streaming-bytes params (io.Reader, io.ReadSeeker, io.ReadCloser)
		// are serialized as bounded byte payloads — they are boundary params,
		// not reconstructed params, even if the report labels them as such.
		if param.Codec == CodecStreamingBytes {
			plan.BoundaryParams = append(plan.BoundaryParams, param)
			continue
		}
		if recon, ok := LookupReconstructor(params.At(i).Type()); ok {
			param.Classification = activation.Reconstructible
			plan.ReconstructedParams = append(plan.ReconstructedParams, ReconstructedParam{
				Param:         param,
				Reconstructor: recon,
			})
			continue
		}
		if param.Codec != CodecPrimitive && reportClassifiesReconstructed(report, param.Name, param.GoType) {
			param.Classification = activation.Reconstructible
			plan.ReconstructedParams = append(plan.ReconstructedParams, ReconstructedParam{Param: param})
			continue
		}
		plan.BoundaryParams = append(plan.BoundaryParams, param)
	}

	results := sig.Results()
	for i := 0; i < results.Len(); i++ {
		result, err := MapResult(i, results.At(i), pkgPath)
		if err != nil {
			return nil, err
		}
		plan.Results = append(plan.Results, result)
	}
	plan.ReturnCodec = ReturnCodecFor(plan.Results)
	return plan, nil
}

func sourceModuleRoot(report reportv2.Report) (string, error) {
	root := strings.TrimSpace(report.BuildConfig.ModuleRoot)
	if root == "" {
		return "", errors.New("codegen: report build config has empty module root")
	}
	if !filepath.IsAbs(root) {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		root = abs
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("codegen: source module root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("codegen: source module root %s is not a directory", root)
	}
	return root, nil
}

func loadPackage(moduleRoot, pkgPath string) (*packages.Package, error) {
	cfg := &packages.Config{
		Dir: moduleRoot,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedModule,
		Fset: token.NewFileSet(),
	}
	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("codegen: package %s not found", pkgPath)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("codegen: package %s has load errors", pkgPath)
	}
	return pkgs[0], nil
}

func modulePath(report reportv2.Report, pkg *packages.Package) string {
	if report.Root.Identity.ModulePath != "" {
		return report.Root.Identity.ModulePath
	}
	if pkg != nil && pkg.Module != nil {
		return pkg.Module.Path
	}
	return ""
}

func packageDir(pkg *packages.Package) string {
	if pkg == nil || len(pkg.GoFiles) == 0 {
		return ""
	}
	return filepath.Dir(pkg.GoFiles[0])
}

func findFunctionPosition(pkg *packages.Package, name, receiver string) token.Position {
	if pkg == nil || pkg.Fset == nil {
		return token.Position{}
	}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Name.Name != name {
				continue
			}
			if receiver != "" {
				if fn.Recv == nil || len(fn.Recv.List) == 0 {
					continue
				}
				if !matchesReceiverType(fn.Recv.List[0].Type, receiver) {
					continue
				}
			}
			return pkg.Fset.Position(fn.Pos())
		}
	}
	return token.Position{}
}

func matchesReceiverType(expr ast.Expr, receiver string) bool {
	base := strings.TrimPrefix(receiver, "*")
	switch t := expr.(type) {
	case *ast.StarExpr:
		return matchesReceiverType(t.X, base)
	case *ast.Ident:
		return t.Name == base
	case *ast.IndexExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == base
		}
	}
	return false
}

func serviceName(report reportv2.Report, funcName string) string {
	if report.Pragma.Name != "" {
		return sanitizeServiceName(report.Pragma.Name)
	}
	return sanitizeServiceName("monolift-" + funcName)
}

var serviceNamePattern = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitizeServiceName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "lifted"
	}
	name = serviceNamePattern.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-_")
	if name == "" {
		return "lifted"
	}
	return strings.ToLower(name)
}

func envServiceName(name string) string {
	name = strings.TrimPrefix(name, "monolift-extracted-")
	name = strings.TrimPrefix(name, "monolift-")
	name = strings.ToUpper(serviceNamePattern.ReplaceAllString(name, "_"))
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return "LIFTED"
	}
	return name
}

func reportClassifiesReconstructed(report reportv2.Report, name, goType string) bool {
	for _, item := range report.State {
		text := strings.ToLower(item.Symbol.ObjectName + " " + item.Disposition + " " + strings.Join(item.Classes, " "))
		if name != "" && strings.Contains(text, strings.ToLower(name)) && strings.Contains(text, "reconstruct") {
			return true
		}
		if goType != "" && strings.Contains(text, strings.ToLower(goType)) && strings.Contains(text, "reconstruct") {
			return true
		}
	}
	for _, dep := range report.ExternalDeps {
		text := strings.ToLower(dep.Identity.ObjectName + " " + dep.AccessPath + " " + dep.ConfigurationSource)
		if name != "" && strings.Contains(text, strings.ToLower(name)) {
			return true
		}
		if goType != "" && strings.Contains(text, strings.ToLower(goType)) {
			return true
		}
	}
	return false
}
