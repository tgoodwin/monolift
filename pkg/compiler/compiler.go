package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/tgoodwin/monolift/pkg/lift"
	"golang.org/x/tools/go/packages"
)

const debugDependencyResolution = true // Set to true to see debug prints

// Compiler holds the parsed ASTs for the application's Go packages.
// It discovers packages within the application's module starting from a root directory.
type Compiler struct {
	Fset *token.FileSet
	// Packages are keyed by their import path (e.g., "my/app/main", "my/app/utils").
	// Each *ast.Package contains the files for that specific package.
	Packages map[string]*ast.Package

	// rootAssignStmt is the root assignment for the current extraction.
	rootAssignStmt *ast.AssignStmt

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

	appPackages := make(map[string]*ast.Package)
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

		if isAppPackage && len(pkg.Syntax) > 0 { // Ensure there are ASTs to process
			astPkg := &ast.Package{
				Name:  pkg.Name,
				Files: make(map[string]*ast.File),
			}
			for i, filePath := range pkg.GoFiles {
				if i < len(pkg.Syntax) { // Should always be true if NeedSyntax is met
					astPkg.Files[filePath] = pkg.Syntax[i]
				} else {
					// This case should ideally not happen if packages.Load is successful
					// and NeedSyntax is properly fulfilled.
					errors = append(errors, fmt.Sprintf("syntax tree missing for file %s in package %s", filePath, pkg.PkgPath))
				}
			}
			if len(astPkg.Files) > 0 {
				appPackages[pkg.PkgPath] = astPkg
				appLoadedPkgs = append(appLoadedPkgs, pkg)
			}
		}
	}

	if len(errors) > 0 {
		fmt.Printf("Warnings during package loading from %s:\n%s\n", appRootPath, strings.Join(errors, "\n"))
		if len(appPackages) == 0 {
			return nil, fmt.Errorf("encountered errors during package loading and found no application packages in %s:\n%s", appRootPath, strings.Join(errors, "\n"))
		}
	}

	if len(appPackages) == 0 {
		return nil, fmt.Errorf("no application Go packages found in %s or its subdirectories", appRootPath)
	}

	compiler := &Compiler{
		Fset:       fset,
		Packages:   appPackages,   // This stores the ASTs
		LoadedPkgs: appLoadedPkgs, // This stores the packages.Package with type info
	}

	return compiler, nil
}

// targetPackageImportPath := ""
// Determine the correct import path for userservice.
// This might be "github.com/tgoodwin/monolift/demo/monolith/userservice" or something based on your go.mod module path + /userservice
// For example, if your go.mod is "my/project", it might be "my/project/demo/monolith/userservice"
// You might need to list `compiler.Packages` keys to find the exact one.
// For now, let's iterate to find it:
// for pkgPath := range appPackages {
// 	if strings.HasSuffix(pkgPath, "socialgraph") { // Adjust this heuristic if needed
// 		targetPackageImportPath = pkgPath
// 		break
// 	}
// }

func (c *Compiler) Compile() error {
	for importPath, astPkg := range c.Packages {
		// fmt.Printf("Found package (import path: %s, name: %s)\n", importPath, astPkg.Name)
		for _, fileAst := range astPkg.Files {
			// fmt.Printf("  File: %s\n", filePath)
			ast.Inspect(fileAst, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					pragmas := GetFuncDeclPragmas(node)
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
									pragmas := GetPragmasFromCommentGroup(docCommentGroup)

									if len(pragmas) > 0 {
										fmt.Printf("    Interface %s has pragmas:\n", typeSpec.Name.Name)
										for _, pragma := range pragmas {
											fmt.Printf("      %s\n", pragma.Raw)
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
												if err != nil {
													fmt.Printf("      [ERROR] Could not automatically find constructor call for %s.%s in main.go: %v\n", constructorPkgPath, constructorName, err)
													fmt.Printf("      HINT: Please add a pragma '// @monolift:instanceFor serviceId=...' to the variable declaration in main.go.\n")
													continue // Skip to the next interface
												}

												// fmt.Printf("      Found constructor call for %s in main.go.\n", constructorName)

												mainPkg, _ := c.getMainPackage()
												rootAssignStmt, err := c.findAssignmentForCallExpr(mainPkg, constructorCall)
												if err != nil {
													fmt.Printf("      [ERROR] Could not find assignment for constructor call %s: %v\n", constructorName, err)
													continue
												}
												// Resolve the full dependency graph for the service
												collectedImports := make(map[string]string)
												instantiationPlan, err := c.resolveDependencies(mainPkg, rootAssignStmt, collectedImports)
												if err != nil {
													fmt.Printf("      [ERROR] Failed to resolve dependencies for %s.%s: %v\n", constructorPkgPath, constructorName, err)
													continue // Skip to the next interface
												}
												// fmt.Printf("      Successfully resolved %d dependencies for %s.\n", len(instantiationPlan.Steps), rootVarName)

												// Add the main interface package to imports if it's not already there
												if _, ok := collectedImports[currentLoadedPkg.PkgPath]; !ok {
													collectedImports[currentLoadedPkg.PkgPath] = currentLoadedPkg.Name
												}

												methodConfigs, err := c.getInterfaceMethodConfigs(typeSpec.Name, currentLoadedPkg)
												if err != nil {
													fmt.Printf("      Error extracting methods for interface %s: %v\n", typeSpec.Name.Name, err)
													continue // Skip to the next interface if methods can't be extracted
												}

												serverStructName := strings.ToLower(typeSpec.Name.Name[:1]) + typeSpec.Name.Name[1:] + "Server"
												delegateFieldName := strings.ToLower(typeSpec.Name.Name[:1]) + typeSpec.Name.Name[1:] + "Delegate"

												templateData := lift.ServerTemplateData{
													InterfacePackageAlias: currentLoadedPkg.Name, // Assumes currentLoadedPkg.Name is suitable as an alias
													InterfacePackagePath:  currentLoadedPkg.PkgPath,
													InterfaceTypeName:     typeSpec.Name.Name,
													ServerStructName:      serverStructName,
													DelegateFieldName:     delegateFieldName,
													Methods:               methodConfigs,
													Imports:               collectedImports,
													InstantiationPlan:     instantiationPlan,
												}

												lift.ExecuteAndPrintTemplate(typeSpec.Name.Name, "output", templateData) // "output" is hardcoded for now
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

	// Example: To access the AST for a specific file in a package:
	// targetImportPath := "your/app/module/pkgname" // Replace with actual import path
	// if appPkg, ok := compiler.Packages[targetImportPath]; ok {
	//   fmt.Printf("Package %s (name: %s) has %d files\n", targetImportPath, appPkg.Name, len(appPkg.Files))
	//   // To get a specific file, you'd need its absolute path:
	//   // targetFilePath := "/absolute/path/to/your/app/module/pkgname/file.go"
	//   // if fileAst, ok := appPkg.Files[targetFilePath]; ok { /* ... use fileAst ... */ }
	// }

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

// getInterfaceMethodConfigs extracts method information from an interface for template generation.
func (c *Compiler) getInterfaceMethodConfigs(ifaceNameIdent *ast.Ident, pkg *packages.Package) ([]lift.MethodConfig, error) {
	var methodDataList []lift.MethodConfig

	ifaceObj := pkg.TypesInfo.Defs[ifaceNameIdent]
	if ifaceObj == nil {
		return nil, fmt.Errorf("could not find type object for interface %s", ifaceNameIdent.Name)
	}

	ifaceTypeName, ok := ifaceObj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("object for %s is not a TypeName", ifaceNameIdent.Name)
	}

	ifaceType, ok := ifaceTypeName.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("type for %s is not an Interface", ifaceNameIdent.Name)
	}

	for i := 0; i < ifaceType.NumExplicitMethods(); i++ {
		method := ifaceType.ExplicitMethod(i)
		methodName := method.Name()
		handlerFuncName := "handle" + strings.ToUpper(methodName[:1]) + methodName[1:]
		httpRoute := "/" + strings.ToLower(methodName)
		methodDataList = append(methodDataList, lift.MethodConfig{Name: methodName, HandlerFuncName: handlerFuncName, HTTPRoute: httpRoute})
	}
	return methodDataList, nil
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

// findAssignmentForCallExpr finds the assignment statement
// where the RHS is the given call expression.
func (c *Compiler) findAssignmentForCallExpr(pkg *packages.Package, targetCall *ast.CallExpr) (*ast.AssignStmt, error) {
	var foundAssign *ast.AssignStmt
	for _, fileAST := range pkg.Syntax {
		ast.Inspect(fileAST, func(n ast.Node) bool {
			if foundAssign != nil {
				return false // Stop searching
			}
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}

			// We only handle assignments where the RHS is a single expression (the targetCall)
			if len(assign.Rhs) != 1 {
				return true
			}

			// Check if the RHS is our target call expression
			if assign.Rhs[0] == targetCall {
				foundAssign = assign
				return false
			}
			return true
		})
		if foundAssign != nil {
			break
		}
	}
	if foundAssign == nil {
		return nil, fmt.Errorf("could not find assignment statement for the given constructor call")
	}
	return foundAssign, nil
}

// resolveDependencies analyzes the given rootCall (an *ast.CallExpr) and recursively
// resolves all its arguments and their sub-dependencies, building an InstantiationPlan.
func (c *Compiler) resolveDependencies(pkg *packages.Package, rootAssignStmt *ast.AssignStmt, imports map[string]string) (*lift.InstantiationPlan, error) {
	c.rootAssignStmt = rootAssignStmt // Set for this compilation run

	// resolvedDeps maps a variable's types.Object.Id() to the Dependency that declares it.
	resolvedDeps := make(map[string]*lift.Dependency)
	depGraph := make(map[*lift.Dependency][]*lift.Dependency)
	var allDeps []*lift.Dependency

	// Start the recursive resolution from the root assignment statement.
	rootDep, err := c.resolveAssignment(pkg, rootAssignStmt, resolvedDeps, depGraph, &allDeps, imports)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root dependency: %w", err)
	}

	// --- Pass 2: Find Relevant Statements that use the variables from Pass 1 ---
	mainFuncBody, err := c.getMainFuncBody(pkg)
	if err != nil {
		return nil, fmt.Errorf("could not find main function body for second pass: %w", err)
	}

	// Create a map of all statements that are already processed as VariableDeclarations for quick lookup.
	varDeclStmts := make(map[ast.Stmt]bool)
	for _, dep := range allDeps {
		if dep.Kind == lift.VariableDeclaration {
			varDeclStmts[dep.OriginalStmt] = true
		}
	}

	for _, stmt := range mainFuncBody.List {
		// Stop processing if we go past the root service declaration.
		if stmt.Pos() > c.rootAssignStmt.Pos() {
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

		// Check if this statement uses any of the variables from our dependency graph.
		usedVars := c.findVarsUsedInStatement(stmt, pkg)
		var relevantPrereqs []*lift.Dependency
		for usedObj := range usedVars {
			if dep, ok := resolvedDeps[usedObj.Id()]; ok {
				relevantPrereqs = append(relevantPrereqs, dep)
			}
		}

		if len(relevantPrereqs) > 0 {
			// This is a relevant statement.
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
			for _, p := range relevantPrereqs {
				if !uniquePrereqs[p] {
					uniquePrereqs[p] = true
					finalPrereqs = append(finalPrereqs, p)
				}
			}
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
	// --- Caching Check ---
	var lhsExprs []ast.Expr
	var rhsExpr ast.Expr

	switch stmt := assignStmt.(type) {
	case *ast.AssignStmt:
		lhsExprs = stmt.Lhs
		rhsExpr = stmt.Rhs[0] // Assuming single RHS for now
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
		VarName:      firstLHSIdent.Name,
		Kind:         lift.VariableDeclaration,
		ProviderID:   fmt.Sprintf("%s-%s", c.Fset.Position(assignStmt.Pos()), c.Fset.Position(assignStmt.End())),
		RenderedForm: renderedAssignment,
		OriginalStmt: assignStmt,
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
				if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil {
					imports[fn.Pkg().Path()] = fn.Pkg().Name()
				}
			}
		} else if selExpr, ok := rhs.Fun.(*ast.SelectorExpr); ok {
			if obj := pkg.TypesInfo.Uses[selExpr.Sel]; obj != nil {
				if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil {
					imports[fn.Pkg().Path()] = fn.Pkg().Name()
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
						imports[importedPkg.Path()] = importedPkg.Name()
					}
				}
			}
		} else if _, ok := rhs.Type.(*ast.Ident); ok {
			// If it's a local type, its package is the current package.
			imports[pkg.PkgPath] = pkg.Name
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
					if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil {
						imports[fn.Pkg().Path()] = fn.Pkg().Name()
					}
				}
			}
		} else if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
					if pkgName, ok := obj.(*types.PkgName); ok {
						imports[pkgName.Imported().Path()] = pkgName.Imported().Name()
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
			// Find the declaration of this variable.
			var varDeclStmt ast.Stmt
			// Search all files in the package for the variable's declaration
			for _, fileAST := range pkg.Syntax {
				ast.Inspect(fileAST, func(node ast.Node) bool {
					if varDeclStmt != nil { // Already found
						return false
					}
					if assign, ok := node.(*ast.AssignStmt); ok {
						for _, lhs := range assign.Lhs {
							if ident, ok := lhs.(*ast.Ident); ok && pkg.TypesInfo.Defs[ident] == v {
								varDeclStmt = assign
								return false // Found it
							}
						}
					} else if decl, ok := node.(*ast.DeclStmt); ok {
						if genDecl, ok := decl.Decl.(*ast.GenDecl); ok {
							for _, spec := range genDecl.Specs {
								if valueSpec, ok := spec.(*ast.ValueSpec); ok {
									for _, name := range valueSpec.Names {
										if pkg.TypesInfo.Defs[name] == v {
											varDeclStmt = decl
											return false // Found it
										}
									}
								}
							}
						}
					}
					return true
				})
				if varDeclStmt != nil {
					break
				}
			}

			if varDeclStmt == nil {
				return nil, fmt.Errorf("could not find declaration/assignment for variable %s", n.Name)
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
			var declStmt ast.Stmt
			for _, fileAST := range pkg.Syntax {
				ast.Inspect(fileAST, func(node ast.Node) bool {
					if declStmt != nil {
						return false
					}
					if decl, ok := node.(*ast.DeclStmt); ok {
						if genDecl, ok := decl.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
							for _, spec := range genDecl.Specs {
								if valueSpec, ok := spec.(*ast.ValueSpec); ok {
									for _, name := range valueSpec.Names {
										if pkg.TypesInfo.Defs[name] == cnst {
											declStmt = decl
											return false
										}
									}
								}
							}
						}
					}
					return true
				})
				if declStmt != nil {
					break
				}
			}
			if declStmt == nil {
				return nil, fmt.Errorf("could not find declaration for constant %s", n.Name)
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

// getMainFuncBody finds the *ast.BlockStmt of the main function in the given package.
func (c *Compiler) getMainFuncBody(pkg *packages.Package) (*ast.BlockStmt, error) {
	if pkg.Name != "main" {
		return nil, fmt.Errorf("package %s is not a main package", pkg.PkgPath)
	}
	for _, fileAST := range pkg.Syntax {
		for _, decl := range fileAST.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				if funcDecl.Name.Name == "main" {
					if funcDecl.Body == nil {
						return nil, fmt.Errorf("main function in package %s has no body", pkg.PkgPath)
					}
					return funcDecl.Body, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("main function not found in package %s", pkg.PkgPath)
}

// exprsToStrings converts a slice of AST expressions to their string representations.
func (c *Compiler) exprsToStrings(exprs []ast.Expr) ([]string, error) {
	var results []string
	for _, expr := range exprs {
		s, err := c.exprToString(expr)
		if err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, nil
}

// stmtToString converts an AST statement back into its Go source code representation.
func (c *Compiler) stmtToString(stmt ast.Stmt) (string, error) {
	var buf bytes.Buffer
	err := printer.Fprint(&buf, c.Fset, stmt)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// exprToString converts an AST expression back into its Go source code representation.
func (c *Compiler) exprToString(expr ast.Expr) (string, error) {
	var buf bytes.Buffer
	err := printer.Fprint(&buf, c.Fset, expr)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// exprToStringDebug converts an AST expression back into its Go source code representation for debugging.
func (c *Compiler) exprToStringDebug(expr ast.Expr) string {
	s, _ := c.exprToString(expr)
	return s
}

// topologicalSort performs a topological sort on the given dependencies.
// It returns an ordered list of dependencies such that each dependency appears
// before any other dependency that relies on it.
func topologicalSort(nodes []*lift.Dependency, graph map[*lift.Dependency][]*lift.Dependency) ([]*lift.Dependency, error) {
	// Kahn's algorithm implementation.
	// `graph` maps a dependency to its *prerequisites*.
	// So, `inDegree[node]` is the count of its prerequisites.
	inDegree := make(map[*lift.Dependency]int)
	for _, node := range nodes {
		inDegree[node] = len(graph[node]) // Number of prerequisites
	}

	queue := []*lift.Dependency{}
	for _, node := range nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	result := []*lift.Dependency{}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		result = append(result, n)

		// Find nodes that *depend on* 'n' (i.e., 'n' is a prerequisite for them)
		// and decrement their in-degree.
		for _, m := range nodes {
			if m == n {
				continue
			}
			for _, prereq := range graph[m] {
				if prereq == n {
					inDegree[m]--
					if inDegree[m] == 0 {
						queue = append(queue, m)
					}
					break // Found 'n' as a prerequisite for 'm', move to next 'm'
				}
			}
		}
	}

	if len(result) != len(nodes) {
		if debugDependencyResolution {
			fmt.Println("--- CIRCULAR DEPENDENCY DETECTED ---")
			fmt.Println("Remaining nodes with unmet dependencies:")
			for node, degree := range inDegree {
				if degree > 0 {
					fmt.Printf("  - Node: %s, In-Degree: %d\n", node, degree)
				}
			}
			fmt.Println("------------------------------------")
		}
		return nil, fmt.Errorf("circular dependency detected in dependency graph")
	}

	return result, nil
}

// findBlockAndIndex finds the *ast.BlockStmt containing targetStmt and the index of targetStmt within that block.
func (c *Compiler) findBlockAndIndex(pkg *packages.Package, targetStmt ast.Stmt) (*ast.BlockStmt, int) {
	var parentBlock *ast.BlockStmt
	var stmtIndex = -1

	for _, fileAST := range pkg.Syntax {
		ast.Inspect(fileAST, func(n ast.Node) bool {
			if block, ok := n.(*ast.BlockStmt); ok {
				for i, stmt := range block.List {
					if stmt == targetStmt {
						parentBlock = block
						stmtIndex = i
						return false // Stop inspection
					}
				}
			}
			return parentBlock == nil // Continue if not found
		})
		if parentBlock != nil {
			break
		}
	}
	return parentBlock, stmtIndex
}

// getVarsDeclaredByStmt returns a map of *types.Var objects for all variables declared on the LHS of a statement.
func (c *Compiler) getVarsDeclaredByStmt(stmt ast.Stmt, pkg *packages.Package) map[types.Object]*types.Var {
	vars := make(map[types.Object]*types.Var)
	ast.Inspect(stmt, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if obj, ok := pkg.TypesInfo.Defs[ident].(*types.Var); ok {
				vars[obj] = obj
			}
		}
		// Only inspect the LHS of assignments/declarations, not the RHS.
		if assign, ok := stmt.(*ast.AssignStmt); ok && n == assign.Rhs[0] {
			return false
		}
		if decl, ok := stmt.(*ast.DeclStmt); ok {
			if vspec, ok := decl.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec); ok && n == vspec.Values[0] {
				return false
			}
		}
		return true
	})
	return vars
}

// findVarsUsedInStatement returns a map of *types.Var objects for all variables used in a statement.
func (c *Compiler) findVarsUsedInStatement(stmt ast.Stmt, pkg *packages.Package) map[types.Object]*types.Var {
	used := make(map[types.Object]*types.Var)
	ast.Inspect(stmt, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if obj, ok := pkg.TypesInfo.Uses[ident].(*types.Var); ok {
				used[obj] = obj
			}
		}
		return true
	})
	return used
}
