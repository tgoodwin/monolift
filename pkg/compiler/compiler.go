package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tgoodwin/monolift/pkg/lift"
	"github.com/tgoodwin/monolift/pkg/pragma"
	"github.com/tgoodwin/monolift/pkg/util"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

const debugDependencyResolution = true // Set to true to see debug prints

const entrypointDirName = "entrypoint"

// extractionResult holds all the necessary information about a service that has been
// identified for extraction.
type extractionResult struct {
	InterfaceTypeName string
	PackageName       string // e.g., "userservice"
	RootStmt          ast.Stmt
	FileAST           *ast.File
	Pragma            *pragma.Pragma
}

// Compiler holds the parsed ASTs for the application's Go packages.
// It discovers packages within the application's module starting from a root directory.
type Compiler struct {
	Fset *token.FileSet

	// rootStmt is the root declaration statement for the current extraction.
	rootStmt ast.Stmt

	// LoadedPkgs stores the original *packages.Package results, which include type information.
	LoadedPkgs []*packages.Package
}

// New parses all Go packages belonging to the application's module, starting from appRootPath.
// It excludes third-party dependencies.
func New(appRootPath string) (*Compiler, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedModule | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  appRootPath, // directory from which to run the go list-like commands.
		Fset: fset,        // for associating AST nodes with the correct file positions.
	}

	// Load all packages within the module found at appRootPath and its subdirectories.
	loadedPkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load packages from %s: %w", appRootPath, err)
	}

	var appLoadedPkgs []*packages.Package
	var errors []string
	for _, pkg := range loadedPkgs {
		for _, pkgErr := range pkg.Errors {
			// TODO: pkg.Errors can contain type errors. Decide how to handle/report them.
			errors = append(errors, fmt.Sprintf("error loading package %s: %v", pkg.PkgPath, pkgErr))
		}

		// Filter for packages that are part of the main module(s) being analyzed.
		// Module.Main is true if this module is the one containing the initial patterns.
		isAppPackage := false
		if pkg.Module != nil && pkg.Module.Main {
			isAppPackage = true
		} else if pkg.Module == nil {
			// Fallback for non-module Go projects (GOPATH mode) or files not in a module.
			// Check if package files are within the appRootPath.
			isContained := true
			if len(pkg.GoFiles) == 0 {
				isContained = false // No source files to consider it local.
			}
			absAppRootPath, _ := filepath.Abs(appRootPath)
			for _, goFilePath := range pkg.GoFiles {
				absGoFilePath, _ := filepath.Abs(goFilePath)
				if !strings.HasPrefix(filepath.Dir(absGoFilePath), absAppRootPath) {
					isContained = false
					break
				}
			}
			if isContained {
				isAppPackage = true
			}
		}

		if isAppPackage && len(pkg.GoFiles) > 0 { // Ensure there are source files to process
			appLoadedPkgs = append(appLoadedPkgs, pkg)
		}
	}

	if len(errors) > 0 {
		fmt.Printf("Warnings during package loading from %s:\n%s\n", appRootPath, strings.Join(errors, "\n"))
		if len(appLoadedPkgs) == 0 {
			return nil, fmt.Errorf("encountered errors during package loading and found no application packages in %s:\n%s", appRootPath, strings.Join(errors, "\n"))
		}
	}

	if len(appLoadedPkgs) == 0 {
		return nil, fmt.Errorf("no application Go packages found in %s or its subdirectories", appRootPath)
	}

	compiler := &Compiler{
		Fset:       fset,
		LoadedPkgs: appLoadedPkgs, // This stores the packages.Package with type info
	}

	return compiler, nil
}

func (c *Compiler) Compile(outputDir, originalAppPath, dockerRegistry, originalK8sManifestPath string) error {
	// 0. clean and create output directories
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("could not remove output directory %s: %w", outputDir, err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("could not create output directory %s: %w", outputDir, err)
	}
	if err := os.MkdirAll(filepath.Join(outputDir, entrypointDirName), 0755); err != nil {
		return fmt.Errorf("could not create entrypoint directory %s: %w", filepath.Join(outputDir, entrypointDirName), err)
	}

	// 1. extract code
	extractedResults, err := c.extractCode(outputDir)
	if err != nil {
		return err
	}
	if len(extractedResults) == 0 {
		fmt.Printf("no @monolift pragmas found.")
		return nil
	}
	fmt.Println("extracted code from the application:")

	// Collect unique package names for the build step.
	var extractedServiceNames []string
	seenNames := make(map[string]bool)
	for _, res := range extractedResults {
		if !seenNames[res.PackageName] {
			extractedServiceNames = append(extractedServiceNames, res.PackageName)
			seenNames[res.PackageName] = true
		}
	}

	// Define Kubernetes-related constants that are needed across different stages.
	namespace := "monolift"
	servicePort := 80 // The port exposed by the Kubernetes Service, not the container's targetPort.

	// 2. Create the entrypoint by recreating the main package in the output directory.
	if err := c.generateEntrypoint(outputDir, extractedResults, namespace, servicePort); err != nil {
		return fmt.Errorf("recreating main package: %w", err)
	}

	// 3. build extracted code artifacts
	builder, _ := newGoBuilder()
	buildArtifacts := append(extractedServiceNames, entrypointDirName)
	if err := builder.build(outputDir, dockerRegistry, buildArtifacts); err != nil {
		return fmt.Errorf("building extracted code artifacts: %w", err)
	}

	// 4. write Kubernetes deployment manifests for the artifacts
	if err := generateK8sManifests(outputDir, dockerRegistry, extractedServiceNames, originalK8sManifestPath, namespace); err != nil {
		return fmt.Errorf("generating Kubernetes manifests: %w", err)
	}

	return nil
}

func (c *Compiler) generateEntrypoint(outputDir string, extracted []*extractionResult, namespace string, servicePort int) error {
	entrypointDir := filepath.Join(outputDir, entrypointDirName)
	fmt.Printf("Recreating main package in %s\n", entrypointDir)

	// Find the main package from the parsed data
	mainPkg, err := c.getMainPackage()
	if err != nil {
		return fmt.Errorf("could not find main package to recreate: %w", err)
	}

	// Group transformations by the file they apply to for efficient processing.
	replacementsByFile := make(map[*ast.File][]*extractionResult)
	for _, res := range extracted {
		replacementsByFile[res.FileAST] = append(replacementsByFile[res.FileAST], res)
	}

	// --- Shared Metrics Monitor Injection ---
	// Check if any extracted service needs a metrics monitor and inject a shared one if so.
	var monitorIdent *ast.Ident
	needsMonitor := false
	for _, res := range extracted {
		// TODO : Check if the pragma requires a metrics monitor.
		if res.Pragma != nil {
			needsMonitor = true
			break
		}
	}

	if needsMonitor {
		mainFunc, err := findFuncDeclInPackage(mainPkg, "main")
		if err != nil {
			return fmt.Errorf("could not find main function to inject metrics monitor: %w", err)
		}

		// Use a unique name to avoid conflicts with user-defined monitors.
		monitorIdent = ast.NewIdent("monoliftMetricsMonitor")
		monitorStmts := generateMonitorStmts(monitorIdent)

		// Prepend the monitor setup statements to the main function's body.
		mainFunc.Body.List = append(monitorStmts, mainFunc.Body.List...)

		// Find the file containing the main function to add imports to it.
		mainFuncFile := findFileForNode(mainPkg, mainFunc)
		if mainFuncFile != nil {
			astutil.AddImport(c.Fset, mainFuncFile, "log")
			astutil.AddImport(c.Fset, mainFuncFile, "time")
			astutil.AddImport(c.Fset, mainFuncFile, "github.com/tgoodwin/monolift/pkg/metrics")
		}
	}

	// Recreate all files from the parsed main package ASTs
	for i, fileAST := range mainPkg.Syntax {
		originalFilePath := mainPkg.GoFiles[i]
		baseName := filepath.Base(originalFilePath)

		// Apply transformations if any are registered for this file.
		if resultsToApply, ok := replacementsByFile[fileAST]; ok {
			fmt.Printf("  Rewriting constructor calls in %s\n", baseName)
			// The delegate block itself only needs the pragma import.
			// Other imports are handled by the shared monitor injection.
			astutil.AddImport(c.Fset, fileAST, "github.com/tgoodwin/monolift/pkg/pragma")

			for _, res := range resultsToApply {
				newStmts, err := generateDelegateBlockStmts(res, namespace, servicePort, monitorIdent)
				if err != nil {
					return fmt.Errorf("failed to generate delegate block for %s: %w", res.PackageName, err)
				}
				if !findAndReplaceStmts(fileAST, res.RootStmt, newStmts) {
					return fmt.Errorf("failed to find and replace root statement for %s in %s", res.PackageName, baseName)
				}
			}
		}

		destPath := filepath.Join(entrypointDir, baseName)

		outFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("could not create destination file %s: %w", destPath, err)
		}
		defer outFile.Close()

		fmt.Printf("  Writing %s\n", destPath)

		// Print the AST to a buffer first.
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, c.Fset, fileAST); err != nil {
			return fmt.Errorf("failed to print AST to buffer: %w", err)
		}

		// Run the generated code through gofmt.
		formattedSrc, err := format.Source(buf.Bytes())
		if err != nil {
			// For debugging, write the unformatted source to see what's wrong.
			_ = os.WriteFile(destPath+".err", buf.Bytes(), 0644)
			return fmt.Errorf("failed to format generated code for %s: %w", baseName, err)
		}

		// Write the formatted code to the file.
		if _, err := outFile.Write(formattedSrc); err != nil {
			return fmt.Errorf("failed to write formatted code to %s: %w", destPath, err)
		}
	}

	entryPointModuleName := "entrypoint"
	if err := util.InitGoMod(entryPointModuleName, entrypointDir); err != nil {
		return fmt.Errorf("could not initialize go.mod in entrypoint directory %s: %w", entrypointDir, err)
	}

	return nil
}

func (c *Compiler) extractCode(outputDir string) ([]*extractionResult, error) {
	extracted := make([]*extractionResult, 0)
	for _, pkg := range c.LoadedPkgs {
		importPath := pkg.PkgPath
		for _, fileAst := range pkg.Syntax {
			ast.Inspect(fileAst, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					pragmas := getFuncDeclPragmas(node)
					if len(pragmas) > 0 {
						fmt.Printf("    Function %s has pragmas:\n", node.Name.Name)
						for _, pragma := range pragmas {
							fmt.Printf("      %s\n", pragma.Raw)
						}
					}
				// ast.GenDecl is used for general declarations like imports, constants, types, etc
				// and includes the comments immediately preceding the declaration.
				case *ast.GenDecl:
					// Check for TYPE declarations (interfaces, structs)
					if node.Tok == token.TYPE {
						for _, spec := range node.Specs {
							if typeSpec, ok := spec.(*ast.TypeSpec); ok {
								if IsInterface(typeSpec) {
									var docCommentGroup *ast.CommentGroup
									if typeSpec.Doc != nil {
										docCommentGroup = typeSpec.Doc
									} else if node.Doc != nil {
										docCommentGroup = node.Doc
									}

									// we currently only support one prama
									pragmas := getPragmasFromCommentGroup(docCommentGroup)

									if len(pragmas) > 0 {
										if len(pragmas) > 1 {
											fmt.Printf("    Warning: Interface %s has multiple pragmas, only the first will be processed.\n", typeSpec.Name.Name)
										}
										pragmaLine := pragmas[0]
										fmt.Printf("    Interface %s has pragma: %s\n", typeSpec.Name.Name, pragmaLine.Raw)

										p, err := pragma.ParsePragma(pragmaLine.Attributes)
										if err != nil {
											fmt.Printf("      Error parsing pragma: %v\n", err)
											continue
										}

										// Now, attempt to find the implementer since it has pragmas
										var currentLoadedPkg *packages.Package
										for _, lp := range c.LoadedPkgs {
											if lp.PkgPath == importPath { // importPath is from the outer loop
												currentLoadedPkg = lp
												break
											}
										}

										if currentLoadedPkg != nil && currentLoadedPkg.TypesInfo != nil {
											implementerNamedType, err := c.findSingleImplementer(typeSpec.Name, currentLoadedPkg)
											if err != nil {
												fmt.Printf("      Error finding implementer for %s.%s: %v\n", importPath, typeSpec.Name.Name, err)
											} else if implementerNamedType != nil {
												fmt.Printf("      Interface %s.%s is implemented by struct %s.%s\n",
													importPath, typeSpec.Name.Name,
													implementerNamedType.Obj().Pkg().Path(), implementerNamedType.Obj().Name())

												// the constructor function naming heuristics are:
												// - New<InterfaceName> for the interface named "Service" would be "NewService"
												// TODO generalize or document this
												constructorName := "New" + typeSpec.Name.Name // e.g., "NewService"
												constructorPkgPath := implementerNamedType.Obj().Pkg().Path()

												constructorCall, _, err := c.findConstructorCallInMain(constructorPkgPath, constructorName)
												// TODO build support for user-provided hints if automatic resolution fails
												if err != nil {
													fmt.Printf("      [ERROR] Could not automatically find constructor call for %s.%s in main.go: %v\n", constructorPkgPath, constructorName, err)
													fmt.Printf("      HINT: Please add a pragma '// @monolift:instanceFor serviceId=...' to the variable declaration in main.go.\n")
													continue // Skip to the next interface
												}

												mainPkg, _ := c.getMainPackage()
												rootStmt, err := c.findDeclStmtForCallExpr(mainPkg, constructorCall)
												if err != nil {
													fmt.Printf("      [ERROR] Could not find assignment for constructor call %s: %v\n", constructorName, err)
													continue
												}
												var fileForStmt *ast.File
												for _, f := range mainPkg.Syntax {
													// A node is in a file if its position is within the file's position range.
													if f.Pos() <= rootStmt.Pos() && rootStmt.End() <= f.End() {
														fileForStmt = f
														break
													}
												}
												// Resolve the full dependency graph for the service
												collectedImports := make(map[string]string)
												instantiationPlan, err := c.resolveDependencies(mainPkg, rootStmt, collectedImports)
												if err != nil {
													fmt.Printf("      [ERROR] Failed to resolve dependencies for %s.%s: %v\n", constructorPkgPath, constructorName, err)
													continue // Skip to the next interface
												}

												// Add the service's own interface package to collectedImports.
												// This ensures it's handled by the general import logic.
												collectedImports[currentLoadedPkg.PkgPath] = util.DetermineImportAlias(currentLoadedPkg.PkgPath, currentLoadedPkg.Name)

												// Split dependencies by scope for the template.
												var pkgScopeDeps, funcScopeDeps []*lift.Dependency

												// instantationPlan.Steps is the topological sort of the dependency graph.
												// we are now splitting dependencies into package and function scope while
												// preserving this order
												for _, dep := range instantiationPlan.Steps {
													// RelevantStatements are always function-scoped.
													// VariableDeclarations can be either.
													if dep.IsPackageScope {
														pkgScopeDeps = append(pkgScopeDeps, dep)
													} else {
														funcScopeDeps = append(funcScopeDeps, dep)
													}
												}

												methodConfigs, err := lift.GetMethodConfigsForInterface(typeSpec.Name, currentLoadedPkg, collectedImports)
												if err != nil {
													fmt.Printf("      Error extracting methods for interface %s: %v\n", typeSpec.Name.Name, err)
													continue // Skip to the next interface if methods can't be extracted
												}

												serverStructName := strings.ToLower(typeSpec.Name.Name[:1]) + typeSpec.Name.Name[1:] + "Server"
												delegateFieldName := strings.ToLower(typeSpec.Name.Name[:1]) + typeSpec.Name.Name[1:] + "Delegate"

												templateData := lift.ServerTemplateData{
													InterfacePackageAlias: currentLoadedPkg.Name,
													InterfacePackagePath:  currentLoadedPkg.PkgPath,
													InterfaceTypeName:     typeSpec.Name.Name,
													ServerStructName:      serverStructName,
													DelegateFieldName:     delegateFieldName,
													Methods:               methodConfigs,
													Imports:               collectedImports,
													PackageScopeDeps:      pkgScopeDeps,
													FunctionScopeDeps:     funcScopeDeps,
													RootDependency:        instantiationPlan.RootDependency,
												}

												// TODO names of extracted code may collide - design a better naming scheme
												// Generate the client for this service
												entrypointDir := filepath.Join(outputDir, entrypointDirName)
												clientData, err := lift.GetClientTemplateData(typeSpec.Name, currentLoadedPkg)
												if err != nil {
													fmt.Printf("      [ERROR] Failed to gather client template data for %s: %v\n", typeSpec.Name.Name, err)
													continue
												}
												if err := lift.ExecuteClientTemplate(entrypointDir, *clientData); err != nil {
													fmt.Printf("      [ERROR] Failed to generate client for %s: %v\n", typeSpec.Name.Name, err)
													continue
												}
												fmt.Printf("      Generated client for %s\n", typeSpec.Name.Name)

												// Generate the delegate for this service
												delegateData, err := lift.GetDelegateTemplateData(typeSpec.Name, currentLoadedPkg)
												if err != nil {
													fmt.Printf("      [ERROR] Failed to gather delegate template data for %s: %v\n", typeSpec.Name.Name, err)
													continue
												}
												if err := lift.ExecuteDelegateTemplate(entrypointDir, *delegateData); err != nil {
													fmt.Printf("      [ERROR] Failed to generate delegate for %s: %v\n", typeSpec.Name.Name, err)
													continue
												}
												fmt.Printf("      Generated delegate for %s\n", typeSpec.Name.Name)

												lift.ExecuteAndPrintTemplate(typeSpec.Name.Name, outputDir, templateData)
												extracted = append(extracted, &extractionResult{
													InterfaceTypeName: typeSpec.Name.Name,
													PackageName:       currentLoadedPkg.Name,
													RootStmt:          rootStmt,
													FileAST:           fileForStmt,
													Pragma:            p,
												})
											}
										} else {
											fmt.Printf("      Could not find loaded package or type info for %s to check implementers for %s\n", importPath, typeSpec.Name.Name)
										}
									} else {
										fmt.Printf("    Interface %s has no @monolift pragmas, skipping implementer search.\n", typeSpec.Name.Name)
									}
								}
							}
						}
					}
				}
				return true // Ensure all code paths return a bool
			})
		}
	}

	return extracted, nil
}

// findAndReplaceStmts finds a statement list containing `oldStmt` and replaces `oldStmt` with `newStmts`.
// It inspects the AST of the given file to find the correct list (e.g., a function body).
func findAndReplaceStmts(file *ast.File, oldStmt ast.Stmt, newStmts []ast.Stmt) bool {
	var replaced bool
	ast.Inspect(file, func(n ast.Node) bool {
		if replaced {
			return false // Stop searching
		}
		var stmts *[]ast.Stmt
		switch x := n.(type) {
		case *ast.BlockStmt:
			stmts = &x.List
		case *ast.File:
			// For top-level declarations, we need to check the file's Decls list.
			// The `oldStmt` will be a `*ast.DeclStmt` wrapping a `*ast.GenDecl`.
			if declStmt, ok := oldStmt.(*ast.DeclStmt); ok {
				for i, decl := range x.Decls {
					if decl == declStmt.Decl {
						// Reconstruct the Decls slice to replace the old one.
						newDecls := make([]ast.Decl, 0, len(x.Decls)-1+len(newStmts))
						newDecls = append(newDecls, x.Decls[:i]...)
						for _, newS := range newStmts {
							if newD, ok := newS.(*ast.DeclStmt); ok {
								newDecls = append(newDecls, newD.Decl)
							} else {
								// This is a problem, we can't put non-declarations at the top level.
								// For now, we assume generateDelegateBlockStmts produces valid top-level stmts.
							}
						}
						newDecls = append(newDecls, x.Decls[i+1:]...)
						x.Decls = newDecls
						replaced = true
						return false
					}
				}
			}
			return true // Not a block, but continue searching children
		default:
			return true // Not a block, continue searching
		}

		for i, s := range *stmts {
			if s == oldStmt {
				// Found the statement to replace.
				// Create a new slice to hold the modified list of statements.
				newStmtList := make([]ast.Stmt, 0, len(*stmts)-1+len(newStmts))
				newStmtList = append(newStmtList, (*stmts)[:i]...)
				newStmtList = append(newStmtList, newStmts...)
				newStmtList = append(newStmtList, (*stmts)[i+1:]...)
				*stmts = newStmtList
				replaced = true
				return false // Stop inner loop and outer inspect
			}
		}
		return true
	})
	return replaced
}

// generateDelegateBlockStmts creates the full block of code that instantiates
// the local service, remote client, metrics monitor, decider, and the final delegate.
func generateDelegateBlockStmts(res *extractionResult, namespace string, port int, monitorIdent *ast.Ident) ([]ast.Stmt, error) {
	// 1. Extract info from the original statement (e.g., `userService := ...`)
	var varIdent *ast.Ident
	var originalCall ast.Expr
	switch s := res.RootStmt.(type) {
	case *ast.AssignStmt:
		varIdent = s.Lhs[0].(*ast.Ident)
		originalCall = s.Rhs[0]
	case *ast.DeclStmt:
		genDecl := s.Decl.(*ast.GenDecl)
		valueSpec := genDecl.Specs[0].(*ast.ValueSpec)
		varIdent = valueSpec.Names[0]
		originalCall = valueSpec.Values[0]
	default:
		return nil, fmt.Errorf("unsupported root statement type: %T", res.RootStmt)
	}

	// 2. Create the `var userService userservice.Service` declaration
	varDecl := &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{ast.NewIdent(varIdent.Name)},
					Type: &ast.SelectorExpr{
						X:   ast.NewIdent(res.PackageName),
						Sel: ast.NewIdent(res.InterfaceTypeName),
					},
				},
			},
		},
	}

	// 3. Build the statements for inside the new `{...}` block
	localSvcIdent := ast.NewIdent("localSvc")
	remoteSvcIdent := ast.NewIdent("remoteSvc")
	deciderIdent := ast.NewIdent("decider")

	// `localSvc := original.NewService(...)`
	localDecl := &ast.AssignStmt{Lhs: []ast.Expr{localSvcIdent}, Tok: token.DEFINE, Rhs: []ast.Expr{originalCall}}

	// `remoteSvc := NewuserserviceClient("http://...")`
	remoteDecl := &ast.AssignStmt{
		Lhs: []ast.Expr{remoteSvcIdent},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun:  ast.NewIdent("New" + res.PackageName + "Client"),
				Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`"http://%s.%s:%d"`, res.PackageName, namespace, port)}},
			},
		},
	}

	// `decider := pragma.NewCPUDecider(monitor, 0.5)`
	deciderDecl := &ast.AssignStmt{
		Lhs: []ast.Expr{deciderIdent},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{X: ast.NewIdent("pragma"), Sel: ast.NewIdent(fmt.Sprintf("New%sDecider", res.Pragma.SignalType))},
				Args: []ast.Expr{
					monitorIdent,
					&ast.BasicLit{Kind: token.FLOAT, Value: strconv.FormatFloat(res.Pragma.Threshold, 'f', -1, 64)},
				},
			},
		},
	}

	// `userService = NewuserserviceDelegate(localSvc, remoteSvc, decider)`
	finalAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(varIdent.Name)},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{&ast.CallExpr{
			Fun:  ast.NewIdent("New" + res.PackageName + "ClientDelegate"),
			Args: []ast.Expr{localSvcIdent, remoteSvcIdent, deciderIdent},
		}},
	}

	// 4. Assemble the final block
	block := &ast.BlockStmt{
		List: []ast.Stmt{localDecl, remoteDecl, deciderDecl, finalAssign},
	}

	return []ast.Stmt{varDecl, block}, nil
}

// generateMonitorStmts creates the AST statements for instantiating and closing a metrics monitor.
// It generates:
//
//	monitor, err := metrics.NewMonitor(1 * time.Second)
//	if err != nil {
//		log.Fatalf("failed to create metrics monitor: %v", err)
//	}
//	defer monitor.Close()
func generateMonitorStmts(monitorIdent *ast.Ident) []ast.Stmt {
	// `monitor, err := metrics.NewMonitor(1 * time.Second)`
	monitorDecl := &ast.AssignStmt{
		Lhs: []ast.Expr{monitorIdent, ast.NewIdent("err")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: &ast.SelectorExpr{X: ast.NewIdent("metrics"), Sel: ast.NewIdent("NewMonitor")}, Args: []ast.Expr{&ast.BinaryExpr{X: &ast.BasicLit{Kind: token.INT, Value: "1"}, Op: token.MUL, Y: &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Second")}}}}},
	}
	// `if err != nil { log.Fatalf(...) }`
	monitorErrCheck := &ast.IfStmt{
		Cond: &ast.BinaryExpr{X: ast.NewIdent("err"), Op: token.NEQ, Y: ast.NewIdent("nil")},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{Fun: &ast.SelectorExpr{X: ast.NewIdent("log"), Sel: ast.NewIdent("Fatalf")}, Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `"failed to create metrics monitor: %v"`}, ast.NewIdent("err")}}}}},
	}
	// `defer monitor.Close()`
	monitorClose := &ast.DeferStmt{Call: &ast.CallExpr{Fun: &ast.SelectorExpr{X: monitorIdent, Sel: ast.NewIdent("Close")}}}

	return []ast.Stmt{monitorDecl, monitorErrCheck, monitorClose}
}

// findFuncDeclInPackage finds a function declaration by name within a given package.
func findFuncDeclInPackage(pkg *packages.Package, funcName string) (*ast.FuncDecl, error) {
	for _, fileAST := range pkg.Syntax {
		for _, decl := range fileAST.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
				return fn, nil
			}
		}
	}
	return nil, fmt.Errorf("function %q not found in package %s", funcName, pkg.PkgPath)
}

// findFileForNode finds the *ast.File that a given ast.Node belongs to within a package.
func findFileForNode(pkg *packages.Package, target ast.Node) *ast.File {
	for _, fileAST := range pkg.Syntax {
		if fileAST.Pos() <= target.Pos() && target.End() <= fileAST.End() {
			return fileAST
		}
	}
	return nil
}

// findSingleImplementer takes an AST identifier for an interface name and its defining package,
// then searches through all loaded application packages to find a single struct type
// that implements this interface.
// If no implementers are found, it returns (nil, error).
// If multiple implementers are found, it returns (nil, error listing them).
// If exactly one implementer is found, it returns the *types.Named for the struct and nil error.
func (c *Compiler) findSingleImplementer(ifaceNameIdent *ast.Ident, definingPkg *packages.Package) (*types.Named, error) {
	if ifaceNameIdent == nil {
		return nil, fmt.Errorf("input interface ast.Ident is nil")
	}
	if definingPkg == nil || definingPkg.TypesInfo == nil {
		return nil, fmt.Errorf("defining package or its TypesInfo is nil for interface %s", ifaceNameIdent.Name)
	}

	// 1. Get the types.Object for the interface definition.
	obj := definingPkg.TypesInfo.Defs[ifaceNameIdent]
	if obj == nil {
		return nil, fmt.Errorf("no type information found for interface identifier %s in package %s", ifaceNameIdent.Name, definingPkg.PkgPath)
	}

	typeName, ok := obj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("%s in package %s is not a type name, but a %T", ifaceNameIdent.Name, definingPkg.PkgPath, obj)
	}

	targetInterface, ok := typeName.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("%s in package %s is not an interface type, but %s", typeName.Name(), definingPkg.PkgPath, typeName.Type().Underlying().String())
	}
	fullInterfaceName := fmt.Sprintf("%s.%s", definingPkg.PkgPath, typeName.Name())

	// 2. Iterate through all loaded packages and their types to find implementers.
	var implementers []*types.Named
	for _, pkgToSearch := range c.LoadedPkgs {
		if pkgToSearch.Types == nil {
			continue
		}
		scope := pkgToSearch.Types.Scope()
		for _, name := range scope.Names() {
			objInScope := scope.Lookup(name)
			if structTypeName, ok := objInScope.(*types.TypeName); ok {
				// Check if it's a named type whose underlying type is a struct.
				if _, isStruct := structTypeName.Type().Underlying().(*types.Struct); isStruct {
					candidateType := structTypeName.Type() // This is the *types.Named for the struct.
					if types.Implements(candidateType, targetInterface) || types.Implements(types.NewPointer(candidateType), targetInterface) {
						if namedCandidate, ok := candidateType.(*types.Named); ok {
							implementers = append(implementers, namedCandidate)
						}
					}
				}
			}
		}
	}

	// 3. Check cardinality of implementers.
	if len(implementers) == 0 {
		return nil, fmt.Errorf("no struct implementer found for interface %s", fullInterfaceName)
	}
	if len(implementers) > 1 {
		var names []string
		for _, impl := range implementers {
			names = append(names, fmt.Sprintf("%s.%s", impl.Obj().Pkg().Path(), impl.Obj().Name()))
		}
		return nil, fmt.Errorf("multiple struct implementers found for interface %s: %s", fullInterfaceName, strings.Join(names, ", "))
	}

	return implementers[0], nil
}

// GetStructMethodASTs finds all *ast.FuncDecl nodes for methods associated with the given structType.
// It searches within the package where structType is defined.
func (c *Compiler) GetStructMethodASTs(structType *types.Named) ([]*ast.FuncDecl, error) {
	if structType == nil {
		return nil, fmt.Errorf("input structType is nil")
	}

	structPkgPath := structType.Obj().Pkg().Path()
	var definingPkgInfo *packages.Package
	for _, lp := range c.LoadedPkgs {
		if lp.PkgPath == structPkgPath {
			definingPkgInfo = lp
			break
		}
	}

	if definingPkgInfo == nil {
		return nil, fmt.Errorf("package %s for struct %s not found in loaded packages", structPkgPath, structType.Obj().Name())
	}
	if definingPkgInfo.TypesInfo == nil {
		return nil, fmt.Errorf("TypesInfo is nil for package %s", structPkgPath)
	}

	var methodDecls []*ast.FuncDecl
	for _, astFile := range definingPkgInfo.Syntax { // definingPkgInfo.Syntax is []*ast.File
		ast.Inspect(astFile, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
				return true // Not a method or no receiver
			}

			// Get the type of the receiver from the AST node
			recvTypeExpr := funcDecl.Recv.List[0].Type
			receiverType := definingPkgInfo.TypesInfo.TypeOf(recvTypeExpr)

			// Check if the receiver type matches the struct type or a pointer to it
			if types.Identical(receiverType, structType) || types.Identical(receiverType, types.NewPointer(structType)) {
				methodDecls = append(methodDecls, funcDecl)
			}
			return true
		})
	}
	return methodDecls, nil
}

func (c *Compiler) getMainPackage() (*packages.Package, error) {
	for _, pkg := range c.LoadedPkgs {
		if pkg.Name == "main" {
			return pkg, nil
		}
	}
	return nil, fmt.Errorf("no 'main' package found in the loaded packages")
}

// findConstructorCallInMain searches the 'main' package of the application for a specific constructor call.
// It returns the AST node for the call expression if found.
func (c *Compiler) findConstructorCallInMain(constructorPkgPath, constructorName string) (*ast.CallExpr, *packages.Package, error) {
	mainPkg, err := c.getMainPackage()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find main package: %w", err)
	}
	var foundCall *ast.CallExpr
	for _, fileAST := range mainPkg.Syntax {
		ast.Inspect(fileAST, func(n ast.Node) bool {
			if foundCall != nil {
				return false // Stop searching once found
			}

			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true // Continue searching
			}

			// The Fun field of a CallExpr can be an *ast.Ident (for direct calls)
			// or an *ast.SelectorExpr (for qualified calls like pkg.Func or obj.Method).
			// We need the *ast.Ident that represents the function name itself.
			var funIdent *ast.Ident
			switch funExpr := callExpr.Fun.(type) {
			case *ast.Ident:
				funIdent = funExpr
			case *ast.SelectorExpr:
				funIdent = funExpr.Sel // This is the 'Func' part in 'pkg.Func' or 'Method' in 'obj.Method'
			default:
				return true // Unexpected type for callExpr.Fun, continue searching
			}
			// Use type information to resolve the function being called.
			obj := mainPkg.TypesInfo.Uses[funIdent]
			if fn, ok := obj.(*types.Func); ok {
				if fn.Pkg() != nil && fn.Pkg().Path() == constructorPkgPath && fn.Name() == constructorName {
					foundCall = callExpr
					return false // Stop searching
				}
			}
			return true
		})

		if foundCall != nil {
			break // Stop iterating through files in the main package
		}
	}

	if foundCall == nil {
		return nil, nil, fmt.Errorf("call to %s.%s not found", constructorPkgPath, constructorName)
	}

	return foundCall, mainPkg, nil
}

// findDeclStmtForCallExpr finds the declaration statement (`:=` or `var =`)
// where the RHS is the given call expression.
func (c *Compiler) findDeclStmtForCallExpr(pkg *packages.Package, targetCall *ast.CallExpr) (ast.Stmt, error) {
	var foundStmt ast.Stmt
	for _, fileAST := range pkg.Syntax {
		ast.Inspect(fileAST, func(n ast.Node) bool {
			if foundStmt != nil {
				return false // Stop searching
			}

			// Check for `:=`
			if assign, ok := n.(*ast.AssignStmt); ok {
				if len(assign.Rhs) == 1 && assign.Rhs[0] == targetCall {
					foundStmt = assign
					return false
				}
			}

			// Check for `var =`
			if decl, ok := n.(*ast.DeclStmt); ok {
				if genDecl, ok := decl.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
					for _, spec := range genDecl.Specs {
						if valueSpec, ok := spec.(*ast.ValueSpec); ok {
							if len(valueSpec.Values) == 1 && valueSpec.Values[0] == targetCall {
								foundStmt = decl
								return false
							}
						}
					}
				}
			}

			return true
		})
		if foundStmt != nil {
			break
		}
	}
	if foundStmt == nil {
		return nil, fmt.Errorf("could not find assignment or declaration statement for the given constructor call")
	}
	return foundStmt, nil
}

// resolveDependencies analyzes the given rootCall (an *ast.CallExpr) and recursively
// resolves all its arguments and their sub-dependencies, building an InstantiationPlan.
func (c *Compiler) resolveDependencies(pkg *packages.Package, rootStmt ast.Stmt, imports map[string]string) (*lift.InstantiationPlan, error) {
	// TODO do not overwrite this - use a dedicated instance of some type for each extraction task
	c.rootStmt = rootStmt // Set for this compilation run

	// resolvedDeps maps a variable's types.Object.Id() to the Dependency that declares it.
	resolvedDeps := make(map[string]*lift.Dependency)
	depGraph := make(map[*lift.Dependency][]*lift.Dependency)
	var allDeps []*lift.Dependency

	// Start the recursive resolution from the root assignment statement.
	rootDep, err := c.resolveAssignment(pkg, rootStmt, resolvedDeps, depGraph, &allDeps, imports)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root dependency: %w", err)
	}

	// --- Pass 2: Find Relevant Statements that use the variables from Pass 1 ---
	executableStmts, err := getExecutableStmtsInOrder(pkg)
	if err != nil {
		return nil, fmt.Errorf("could not find executable statements for second pass: %w", err)
	}

	// Create a map of all statements that are already processed as VariableDeclarations for quick lookup.
	varDeclStmts := make(map[ast.Stmt]bool)
	for _, dep := range allDeps {
		if dep.Kind == lift.VariableDeclaration {
			varDeclStmts[dep.OriginalStmt] = true
		}
	}

	for _, stmt := range executableStmts {
		// Stop processing if we go past the root service declaration.
		if stmt.Pos() > c.rootStmt.Pos() {
			break
		}

		// Skip statements we've already processed as variable declarations.
		if varDeclStmts[stmt] {
			continue
		}

		// Heuristic: Only consider control flow or expression statements for relevance.
		isControlFlow := false
		isExprStmt := false
		switch stmt.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt:
			isControlFlow = true
		case *ast.ExprStmt:
			isExprStmt = true
		}
		if !isControlFlow && !isExprStmt {
			continue
		}

		// Check if this statement uses any of the variables from our dependency graph to determine relevance.
		usedIdents := findVarIdentsUsedInStatement(stmt, pkg)
		isRelevant := false
		for _, ident := range usedIdents {
			if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
				if _, ok := resolvedDeps[obj.Id()]; ok {
					isRelevant = true
					break
				}
			}
		}

		if isRelevant {
			// This is a relevant statement. Resolve all variables it uses to form its prerequisites.
			var allPrereqs []*lift.Dependency
			processedVars := make(map[string]bool)

			for _, ident := range usedIdents {
				obj := pkg.TypesInfo.Uses[ident]
				if obj == nil || processedVars[obj.Id()] {
					continue
				}
				processedVars[obj.Id()] = true // Mark as processed for this statement

				// resolveExpr finds the declaration of the variable `ident` refers to,
				// adds it to the dependency graph if needed, and returns the dependency.
				dep, err := c.resolveExpr(pkg, ident, resolvedDeps, depGraph, &allDeps, imports)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve variable '%s' used in a relevant statement: %w", ident.Name, err)
				}
				if dep != nil {
					allPrereqs = append(allPrereqs, dep)
				}
			}

			renderedStmt, err := c.stmtToString(stmt)
			if err != nil {
				return nil, fmt.Errorf("failed to render relevant statement to string: %w", err)
			}

			relevantDep := &lift.Dependency{
				Kind:         lift.RelevantStatement,
				ProviderID:   fmt.Sprintf("%s-%s", c.Fset.Position(stmt.Pos()), c.Fset.Position(stmt.End())),
				RenderedForm: renderedStmt,
				OriginalStmt: stmt,
			}

			// Ensure unique prerequisites.
			uniquePrereqs := make(map[*lift.Dependency]bool)
			finalPrereqs := []*lift.Dependency{}
			for _, p := range allPrereqs {
				if !uniquePrereqs[p] {
					uniquePrereqs[p] = true
					finalPrereqs = append(finalPrereqs, p)
				}
			}
			// The prerequisites are now all resolved dependencies for the variables used.
			depGraph[relevantDep] = finalPrereqs
			allDeps = append(allDeps, relevantDep)
			c.collectImportsFromStmt(pkg, stmt, imports)
		}
	}

	// Perform topological sort to get the correct instantiation order.
	orderedDeps, err := topologicalSort(allDeps, depGraph)
	if err != nil {
		return nil, fmt.Errorf("failed to topologically sort dependencies: %w", err)
	}

	return &lift.InstantiationPlan{Steps: orderedDeps, RootDependency: rootDep}, nil
}

// resolveAssignment resolves an assignment statement (e.g., `x := f(y)` or `var x = f(y)`).
// It returns the created lift.Dependency for the assigned variable.
func (c *Compiler) resolveAssignment(
	pkg *packages.Package,
	assignStmt ast.Stmt, // Can be *ast.AssignStmt or *ast.DeclStmt
	resolvedDeps map[string]*lift.Dependency,
	depGraph map[*lift.Dependency][]*lift.Dependency,
	allDeps *[]*lift.Dependency,
	imports map[string]string,
) (*lift.Dependency, error) {
	var lhsExprs []ast.Expr
	var rhsExpr ast.Expr

	switch stmt := assignStmt.(type) {
	case *ast.AssignStmt:
		lhsExprs = stmt.Lhs
		rhsExpr = stmt.Rhs[0] // Only supporting single RHS for now, e.g. `x := f(y)`
	case *ast.DeclStmt:
		genDecl := stmt.Decl.(*ast.GenDecl)
		valueSpec := genDecl.Specs[0].(*ast.ValueSpec) // Assuming single ValueSpec for now
		// convert []*ast.Ident to []ast.Expr
		lhsExprs = make([]ast.Expr, len(valueSpec.Names))
		for i, ident := range valueSpec.Names {
			lhsExprs[i] = ident
		}
		rhsExpr = valueSpec.Values[0] // Assuming single RHS for now
	default:
		return nil, fmt.Errorf("unsupported assignment statement type: %T", assignStmt)
	}

	// Get the name of the primary variable being declared (the first one on LHS)
	firstLHSIdent, ok := lhsExprs[0].(*ast.Ident)
	if !ok {
		return nil, fmt.Errorf("first LHS of assignment is not an identifier: %T", lhsExprs[0])
	}

	// Use the types.Object of the first declared variable as the canonical key for this dependency.
	obj := pkg.TypesInfo.Defs[firstLHSIdent]
	if obj == nil {
		return nil, fmt.Errorf("could not find type object for identifier %s", firstLHSIdent.Name)
	}
	providerID := obj.Id()
	if dep, ok := resolvedDeps[providerID]; ok {
		return dep, nil // Return cached dependency
	}

	// Render the full assignment statement to a string for the template
	renderedAssignment, err := c.stmtToString(assignStmt)
	if err != nil {
		return nil, fmt.Errorf("failed to render assignment statement to string: %w", err)
	}

	// Create the Dependency object for this variable declaration
	dep := &lift.Dependency{
		VarName:        firstLHSIdent.Name,
		Kind:           lift.VariableDeclaration,
		ProviderID:     fmt.Sprintf("%s-%s", c.Fset.Position(assignStmt.Pos()), c.Fset.Position(assignStmt.End())),
		RenderedForm:   renderedAssignment,
		OriginalStmt:   assignStmt,
		IsPackageScope: isPackageScope(pkg, assignStmt),
	}
	*allDeps = append(*allDeps, dep)

	// Add all variables declared by this statement to the cache, pointing to this single dependency.
	for _, lhs := range lhsExprs {
		if ident, ok := lhs.(*ast.Ident); ok {
			if defObj := pkg.TypesInfo.Defs[ident]; defObj != nil {
				resolvedDeps[defObj.Id()] = dep
			}
		}
	}

	// Now, recursively resolve the arguments/components of the RHS expression
	// This is where we build the graph of prerequisites.
	var prereqs []*lift.Dependency
	switch rhs := rhsExpr.(type) {
	case *ast.CallExpr:
		// For a function call, resolve each argument.
		for _, argExpr := range rhs.Args {
			prereqDep, err := c.resolveExpr(pkg, argExpr, resolvedDeps, depGraph, allDeps, imports)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve argument for call %s: %w", c.exprToStringDebug(argExpr), err)
			}
			if prereqDep != nil { // Only add if it's a declarable dependency
				prereqs = append(prereqs, prereqDep)
			}
		}
		// Add the function's package to imports
		if funIdent, ok := rhs.Fun.(*ast.Ident); ok {
			if obj := pkg.TypesInfo.Uses[funIdent]; obj != nil {
				if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() != pkg.PkgPath {
					imports[fn.Pkg().Path()] = util.DetermineImportAlias(fn.Pkg().Path(), fn.Pkg().Name())
				}
			}
		} else if selExpr, ok := rhs.Fun.(*ast.SelectorExpr); ok {
			if obj := pkg.TypesInfo.Uses[selExpr.Sel]; obj != nil {
				if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() != pkg.PkgPath {
					imports[fn.Pkg().Path()] = util.DetermineImportAlias(fn.Pkg().Path(), fn.Pkg().Name())
				}
			}
		}

	case *ast.CompositeLit:
		// For a struct literal, resolve each field's value.
		for _, elt := range rhs.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				prereqDep, err := c.resolveExpr(pkg, kv.Value, resolvedDeps, depGraph, allDeps, imports)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve field %s for struct literal: %w", c.exprToStringDebug(kv.Key), err)
				}
				if prereqDep != nil {
					prereqs = append(prereqs, prereqDep)
				}
			}
		}
		// Add the struct's package to imports
		if selExpr, ok := rhs.Type.(*ast.SelectorExpr); ok {
			if ident, ok := selExpr.X.(*ast.Ident); ok {
				if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
					if pkgName, ok := obj.(*types.PkgName); ok {
						importedPkg := pkgName.Imported()
						imports[importedPkg.Path()] = util.DetermineImportAlias(importedPkg.Path(), importedPkg.Name())
					}
				}
			}
		}

	case *ast.Ident: // RHS is a variable reference (e.g., `x := y`)
		// Resolve 'y' to find its declaration.
		prereqDep, err := c.resolveExpr(pkg, rhs, resolvedDeps, depGraph, allDeps, imports)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve RHS identifier %s: %w", c.exprToStringDebug(rhs), err)
		}
		if prereqDep != nil {
			prereqs = append(prereqs, prereqDep)
		}

	// For basic literals or selector expressions (like `somepkg.SomeConst`),
	// they are inlined and don't have further declarable dependencies.
	case *ast.BasicLit, *ast.SelectorExpr:
		// No further declarable dependencies to resolve.

	default:
		return nil, fmt.Errorf("unsupported RHS expression type for assignment: %T", rhsExpr)
	}

	if debugDependencyResolution {
		var prereqNames []string
		for _, p := range prereqs {
			prereqNames = append(prereqNames, p.String())
		}
		fmt.Printf("[DEBUG] Adding to graph: %s depends on %v\n", dep, prereqNames)
	}

	depGraph[dep] = prereqs // Add prerequisites to the graph

	return dep, nil
}

// findDeclStmtForVar searches through the package's files to find the declaration
// statement (*ast.AssignStmt or *ast.DeclStmt) for a given variable or constant object.
// For top-level `var` or `const` declarations (*ast.GenDecl), it returns a synthetic *ast.DeclStmt.
func findDeclStmtForVar(pkg *packages.Package, v types.Object) (ast.Stmt, error) {
	var foundStmt ast.Stmt
	for _, fileAST := range pkg.Syntax {
		ast.Inspect(fileAST, func(node ast.Node) bool {
			if foundStmt != nil {
				return false // Stop searching once found
			}
			switch n := node.(type) {
			case *ast.AssignStmt:
				for _, lhs := range n.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && pkg.TypesInfo.Defs[ident] == v {
						foundStmt = n
						return false // Found it
					}
				}
				return true // Continue searching children
			case *ast.DeclStmt:
				// This handles `var` or `const` inside a function.
				if genDecl, ok := n.Decl.(*ast.GenDecl); ok {
					for _, spec := range genDecl.Specs {
						if valueSpec, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range valueSpec.Names {
								if pkg.TypesInfo.Defs[name] == v {
									foundStmt = n
									return false // Found it
								}
							}
						}
					}
				}
				// We've handled this DeclStmt, don't inspect its children (the GenDecl).
				return false
			case *ast.GenDecl:
				// This must be a top-level GenDecl, since we stop traversal at DeclStmt.
				for _, spec := range n.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if pkg.TypesInfo.Defs[name] == v {
								// It's a top-level decl. Wrap it in a synthetic DeclStmt.
								foundStmt = &ast.DeclStmt{Decl: n}
								return false // Found it
							}
						}
					}
				}
				return true // Continue searching children
			}
			return true
		})
		if foundStmt != nil {
			return foundStmt, nil
		}
	}
	return nil, fmt.Errorf("declaration not found")
}

// collectImportsFromStmt recursively collects imports from a statement.
// This is used for statements that are copied as a whole (RelevantStatement).
func (c *Compiler) collectImportsFromStmt(pkg *packages.Package, stmt ast.Stmt, imports map[string]string) {
	ast.Inspect(stmt, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			var funIdent *ast.Ident
			switch funExpr := call.Fun.(type) {
			case *ast.Ident:
				funIdent = funExpr
			case *ast.SelectorExpr:
				funIdent = funExpr.Sel
			}
			if funIdent != nil {
				if obj := pkg.TypesInfo.Uses[funIdent]; obj != nil {
					if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() != pkg.PkgPath { // Don't import current package
						imports[fn.Pkg().Path()] = util.DetermineImportAlias(fn.Pkg().Path(), fn.Pkg().Name())
					}
				}
			}
		} else if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
					if pkgName, ok := obj.(*types.PkgName); ok { // This is a package qualifier (e.g., `fmt.Println`)
						importedPkg := pkgName.Imported()
						if importedPkg.Path() != pkg.PkgPath { // Don't import current package
							imports[importedPkg.Path()] = util.DetermineImportAlias(importedPkg.Path(), importedPkg.Name())
						}
					}
				}
			}
		}
		return true
	})
}

// resolveExpr recursively resolves an AST expression that is *not* the RHS of a variable declaration.
// It returns a non-nil *lift.Dependency if the expression itself needs to be declared as a variable
// (e.g., if it's a reference to another variable that needs to be declared).
// Otherwise, it returns nil, indicating the expression can be inlined.
func (c *Compiler) resolveExpr(
	pkg *packages.Package,
	expr ast.Expr,
	resolvedDeps map[string]*lift.Dependency,
	depGraph map[*lift.Dependency][]*lift.Dependency,
	allDeps *[]*lift.Dependency,
	imports map[string]string,
) (*lift.Dependency, error) {
	// Get the type information for the expression.
	exprType := pkg.TypesInfo.TypeOf(expr)
	if exprType == nil { // Can be nil for untyped expressions like `nil`
		// Check if it's the untyped nil literal
		if ident, ok := expr.(*ast.Ident); ok && pkg.TypesInfo.Uses[ident] == nil && ident.Name == "nil" {
			// This is the untyped nil literal, which has no type. It's an inlined expression.
			return nil, nil
		}
		// For any other untyped expression, we can't proceed.
		return nil, fmt.Errorf("could not determine type of expression: %T (value: %s)", expr, c.exprToStringDebug(expr))
	}

	// Generate a unique ID for this expression to use as a cache key.
	var providerID string
	var obj types.Object

	switch n := expr.(type) {
	case *ast.Ident: // This is a reference to a variable or constant (e.g., `myVar`, `MyConst`)
		obj = pkg.TypesInfo.Uses[n]
		if obj == nil {
			return nil, fmt.Errorf("could not find object for identifier %s", n.Name)
		}
		providerID = obj.Id() // Use the variable's unique ID as the cache key

		// Check cache first for identifiers
		if dep, ok := resolvedDeps[providerID]; ok {
			return dep, nil // Return cached declarable dependency
		}

		// If it's a variable or constant, we need to find its declaration and resolve its RHS.
		if v, isVar := obj.(*types.Var); isVar {
			varDeclStmt, err := findDeclStmtForVar(pkg, v)
			if err != nil {
				return nil, fmt.Errorf("could not find declaration/assignment for variable %s: %w", n.Name, err)
			}

			// Recursively resolve the assignment statement.
			resolvedDep, err := c.resolveAssignment(pkg, varDeclStmt, resolvedDeps, depGraph, allDeps, imports)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve declaration for variable %s: %w", n.Name, err)
			}
			return resolvedDep, nil // Return the declarable dependency
		}
		// If it's a constant, treat it similarly to a variable: find its declaration.
		if cnst, isConst := obj.(*types.Const); isConst {
			declStmt, err := findDeclStmtForVar(pkg, cnst)
			if err != nil {
				return nil, fmt.Errorf("could not find declaration for constant %s: %w", n.Name, err)
			}
			resolvedDep, err := c.resolveAssignment(pkg, declStmt, resolvedDeps, depGraph, allDeps, imports)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve declaration for constant %s: %w", n.Name, err)
			}
			return resolvedDep, nil
		}
		return nil, fmt.Errorf("unhandled identifier type: %T for %s", obj, n.Name)

	case *ast.CallExpr, *ast.BasicLit, *ast.CompositeLit, *ast.SelectorExpr:
		// These are all inlined expressions and do not create a declarable dependency themselves.
		return nil, nil

	default:
		return nil, fmt.Errorf("unsupported AST expression type for dependency resolution: %T", expr)
	}
}

// generateK8sManifests creates Kubernetes Deployment and Service YAMLs for each extracted service.
// It currently focuses on extracted services and does not generate manifests for the entrypoint.
func generateK8sManifests(outputDir, dockerRegistry string, extractedServiceNames []string, originalK8sManifestPath, namespace string) error {
	fmt.Println("\nGenerating Kubernetes manifests:")
	containerPort := 8080

	var envVars []lift.EnvVar
	if originalK8sManifestPath != "" {
		var err error
		envVars, err = extractEnvVarsFromK8sManifest(originalK8sManifestPath)
		if err != nil {
			return fmt.Errorf("failed to extract environment variables from original K8s manifest: %w", err)
		}
		if len(envVars) > 0 {
			fmt.Printf("  Extracted %d environment variables from %s\n", len(envVars), originalK8sManifestPath)
		}
	}

	for _, serviceName := range extractedServiceNames {
		fmt.Printf("  Generating K8s manifests for service %s\n", serviceName)
		imageName := fmt.Sprintf("%s/%s:latest", dockerRegistry, serviceName)
		if err := lift.GenerateExtractedServiceManifests(outputDir, serviceName, namespace, imageName, containerPort, envVars); err != nil {
			return fmt.Errorf("failed to generate K8s service manifest for %s: %w", serviceName, err)
		}
	}

	// Generate manifests for the entrypoint application.
	if originalK8sManifestPath != "" {
		fmt.Println("  Generating K8s manifests for the entrypoint application")
		entrypointImageName := fmt.Sprintf("%s/%s:latest", dockerRegistry, entrypointDirName)

		originalManifestData, err := os.ReadFile(originalK8sManifestPath)
		if err != nil {
			return fmt.Errorf("failed to read original entrypoint manifest %s: %w", originalK8sManifestPath, err)
		}

		if err := lift.GenerateEntrypointManifests(outputDir, namespace, entrypointImageName, originalManifestData, containerPort); err != nil {
			return fmt.Errorf("failed to generate K8s entrypoint manifests: %w", err)
		}
	}

	return nil
}
