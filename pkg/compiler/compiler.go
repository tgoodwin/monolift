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

// Compiler holds the parsed ASTs for the application's Go packages.
// It discovers packages within the application's module starting from a root directory.
type Compiler struct {
	Fset *token.FileSet
	// Packages are keyed by their import path (e.g., "my/app/main", "my/app/utils").
	// Each *ast.Package contains the files for that specific package.
	Packages map[string]*ast.Package

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
												rootVarName, err := c.findVarForCallExpr(mainPkg, constructorCall)
												if err != nil {
													fmt.Printf("      [ERROR] Could not find variable for constructor call %s: %v\n", constructorName, err)
													continue
												}
												// Resolve the full dependency graph for the service
												collectedImports := make(map[string]string)
												instantiationPlan, err := c.resolveDependencies(mainPkg, constructorCall, rootVarName, collectedImports)
												if err != nil {
													fmt.Printf("      [ERROR] Failed to resolve dependencies for %s.%s: %v\n", constructorPkgPath, constructorName, err)
													continue // Skip to the next interface
												}
												fmt.Printf("      Successfully resolved %d dependencies for %s.\n", len(instantiationPlan.Steps), rootVarName)

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

// findVarForCallExpr finds the variable name on the LHS of an assignment
// where the RHS is the given call expression.
func (c *Compiler) findVarForCallExpr(pkg *packages.Package, targetCall *ast.CallExpr) (string, error) {
	var varName string
	for _, fileAST := range pkg.Syntax {
		ast.Inspect(fileAST, func(n ast.Node) bool {
			if varName != "" {
				return false // Stop searching
			}
			// Look for `varName := ...` or `var varName = ...`
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}

			// We only handle simple assignments `var := call()`
			if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
				return true
			}

			// Check if the RHS is our target call expression
			if assign.Rhs[0] == targetCall {
				if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
					varName = ident.Name
					return false
				}
			}
			return true
		})
		if varName != "" {
			break
		}
	}
	if varName == "" {
		return "", fmt.Errorf("could not find variable assignment for the given constructor call")
	}
	return varName, nil
}

// resolveDependencies analyzes the given rootCall (an *ast.CallExpr) and recursively
// resolves all its arguments and their sub-dependencies, building an InstantiationPlan.
func (c *Compiler) resolveDependencies(pkg *packages.Package, rootCall *ast.CallExpr, rootVarName string, imports map[string]string) (*lift.InstantiationPlan, error) {
	// resolvedDeps maps types.Object.Id() to *Dependency to cache and avoid re-processing.
	resolvedDeps := make(map[string]*lift.Dependency)
	// depGraph maps a dependency to the list of dependencies it directly relies on.
	depGraph := make(map[*lift.Dependency][]*lift.Dependency)
	var allDeps []*lift.Dependency

	// Start the recursive resolution from the root call, providing its known variable name.
	_, rootDep, err := c.resolveExpr(pkg, rootCall, resolvedDeps, depGraph, &allDeps, imports, rootVarName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root dependency: %w", err)
	}

	// Perform topological sort to get the correct instantiation order.
	orderedDeps, err := topologicalSort(allDeps, depGraph)
	if err != nil {
		return nil, fmt.Errorf("failed to topologically sort dependencies: %w", err)
	}

	return &lift.InstantiationPlan{Steps: orderedDeps, RootDependency: rootDep}, nil
}

// resolveExpr recursively resolves an AST expression.
// It returns the string representation of the expression for use as an argument,
// and a non-nil *lift.Dependency if the expression must be declared as a variable.
// `desiredVarName` is passed when the expression is known to be the RHS of an assignment.
func (c *Compiler) resolveExpr(
	pkg *packages.Package,
	expr ast.Expr,
	resolvedDeps map[string]*lift.Dependency,
	depGraph map[*lift.Dependency][]*lift.Dependency,
	allDeps *[]*lift.Dependency,
	imports map[string]string,
	desiredVarName string,
) (string, *lift.Dependency, error) {
	// Get the type information for the expression.
	exprType := pkg.TypesInfo.TypeOf(expr) // Can be nil for untyped expressions like `nil`
	if exprType == nil {
		// Allow untyped nil literal
		if ident, ok := expr.(*ast.Ident); ok && ident.Name == "nil" {
			// ok
		} else {
			return "", nil, fmt.Errorf("could not determine type of expression: %T", expr)
		}
	}

	// Determine if the expression is a pointer type.
	isPointer := exprType != nil && types.IsInterface(exprType.Underlying())

	// Generate a unique ID for this expression to use as a cache key.
	// For identifiers, use their types.Object ID. For literals/calls, use their position.
	var providerID string
	var obj types.Object

	switch n := expr.(type) {
	case *ast.Ident:
		obj = pkg.TypesInfo.Uses[n]
		if obj == nil {
			return "", nil, fmt.Errorf("could not find object for identifier %s", n.Name)
		}
		providerID = obj.Id()

		// Check cache first for identifiers
		if dep, ok := resolvedDeps[providerID]; ok {
			return dep.VarName, dep, nil
		}

		// If it's a variable, we need to find its declaration and resolve its RHS.
		if v, ok := obj.(*types.Var); ok {
			// Find the declaration of this variable.
			// This is a simplified approach; a full solution might need to trace data flow.
			// For now, we assume it's declared in the same function/scope.
			// We'll look for an assignment statement or declaration statement.
			var rhsExpr ast.Expr
			ast.Inspect(pkg.Syntax[0], func(node ast.Node) bool { // Inspecting only the first file for simplicity
				if assign, ok := node.(*ast.AssignStmt); ok {
					for i, lhs := range assign.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok && pkg.TypesInfo.Defs[ident] == v {
							rhsExpr = assign.Rhs[i]
							return false // Found it
						}
					}
				} else if decl, ok := node.(*ast.DeclStmt); ok {
					if genDecl, ok := decl.Decl.(*ast.GenDecl); ok {
						for _, spec := range genDecl.Specs {
							if valueSpec, ok := spec.(*ast.ValueSpec); ok {
								for i, name := range valueSpec.Names {
									if pkg.TypesInfo.Defs[name] == v {
										if len(valueSpec.Values) > i {
											rhsExpr = valueSpec.Values[i]
										}
										return false // Found it
									}
								}
							}
						}
					}
				}
				return true
			})

			if rhsExpr == nil {
				return "", nil, fmt.Errorf("could not find declaration/assignment for variable %s", n.Name)
			}
			// Recursively resolve the RHS.
			_, resolvedDep, err := c.resolveExpr(pkg, rhsExpr, resolvedDeps, depGraph, allDeps, imports, n.Name)
			if err != nil {
				return "", nil, fmt.Errorf("failed to resolve RHS for variable %s: %w", n.Name, err)
			}
			resolvedDeps[providerID] = resolvedDep
			return n.Name, resolvedDep, nil
		}
		// If it's a constant, treat as literal.
		if _, ok := obj.(*types.Const); ok {
			return n.Name, nil, nil
		}
		return "", nil, fmt.Errorf("unhandled identifier type: %T for %s", obj, n.Name)

	case *ast.CallExpr:
		// If this call is not being assigned to a variable, we treat it as an inline expression.
		if desiredVarName == "" {
			renderable, err := c.exprToString(n)
			return renderable, nil, err
		}

		// Resolve the function being called.
		var funIdent *ast.Ident
		switch funExpr := n.Fun.(type) {
		case *ast.Ident:
			funIdent = funExpr
		case *ast.SelectorExpr:
			funIdent = funExpr.Sel
		default:
			return "", nil, fmt.Errorf("unsupported function expression type: %T", n.Fun)
		}
		funObj := pkg.TypesInfo.Uses[funIdent]
		if funObj == nil {
			return "", nil, fmt.Errorf("could not find object for function call %s", funIdent.Name)
		}
		fun := funObj.(*types.Func)

		if fun.Pkg() != nil {
			imports[fun.Pkg().Path()] = fun.Pkg().Name()
		}

		// This is a dependency that needs to be declared.
		providerID = fmt.Sprintf("%s-%s", pkg.Fset.Position(n.Pos()), pkg.Fset.Position(n.End()))
		if dep, ok := resolvedDeps[providerID]; ok {
			return dep.VarName, dep, nil
		}

		var args []string
		var prereqs []*lift.Dependency
		for i, argExpr := range n.Args {
			renderable, prereqDep, err := c.resolveExpr(pkg, argExpr, resolvedDeps, depGraph, allDeps, imports, "") // No desired name for args
			if err != nil {
				return "", nil, fmt.Errorf("failed to resolve argument %d for call %s: %w", i, fun.Name(), err)
			}
			args = append(args, renderable)
			if prereqDep != nil {
				prereqs = append(prereqs, prereqDep)
			}
		}

		dep := &lift.Dependency{
			VarName:    desiredVarName,
			VarType:    exprType,
			Kind:       lift.ConstructorCall,
			ProviderID: providerID,
			IsPointer:  isPointer,
			CtorCallData: &lift.ConstructorCallData{
				PkgPath:  fun.Pkg().Path(),
				PkgName:  fun.Pkg().Name(),
				FuncName: fun.Name(),
				Args:     args,
			},
		}
		resolvedDeps[providerID] = dep
		depGraph[dep] = prereqs // Add dependencies to the graph
		*allDeps = append(*allDeps, dep)
		return desiredVarName, dep, nil

	case *ast.BasicLit:
		// Basic literals are always inlined.
		return n.Value, nil, nil

	case *ast.CompositeLit:
		// If this struct literal is not being assigned to a variable, we treat it as an inline expression.
		if desiredVarName == "" {
			renderable, err := c.exprToString(n)
			return renderable, nil, err
		}

		// Resolve type of the composite literal
		var typeName string
		var typePkgName string
		var typePkgPath string
		if selExpr, ok := n.Type.(*ast.SelectorExpr); ok {
			typeName = selExpr.Sel.Name
			if ident, ok := selExpr.X.(*ast.Ident); ok {
				obj := pkg.TypesInfo.Uses[ident]
				if pkgName, ok := obj.(*types.PkgName); ok {
					importedPkg := pkgName.Imported()
					typePkgPath = importedPkg.Path()
					typePkgName = importedPkg.Name()
					imports[typePkgPath] = typePkgName
				}
			}
		} else if ident, ok := n.Type.(*ast.Ident); ok {
			typeName = ident.Name
			typePkgName = pkg.Name
			typePkgPath = pkg.PkgPath // Assume same package if not qualified
		} else {
			return "", nil, fmt.Errorf("unsupported composite literal type expression: %T", n.Type)
		}

		// This is a dependency that needs to be declared.
		providerID = fmt.Sprintf("%s-%s", pkg.Fset.Position(n.Pos()), pkg.Fset.Position(n.End()))
		if dep, ok := resolvedDeps[providerID]; ok {
			return dep.VarName, dep, nil
		}

		fieldValues := make(map[string]string)
		var prereqs []*lift.Dependency
		for _, elt := range n.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				fieldName := kv.Key.(*ast.Ident).Name
				renderable, prereqDep, err := c.resolveExpr(pkg, kv.Value, resolvedDeps, depGraph, allDeps, imports, "")
				if err != nil {
					return "", nil, fmt.Errorf("failed to resolve field %s for struct literal: %w", fieldName, err)
				}
				fieldValues[fieldName] = renderable
				if prereqDep != nil {
					prereqs = append(prereqs, prereqDep)
				}
			} else {
				return "", nil, fmt.Errorf("unsupported element in composite literal: %T", elt)
			}
		}

		dep := &lift.Dependency{
			VarName:    desiredVarName,
			VarType:    exprType,
			Kind:       lift.StructLiteral,
			ProviderID: providerID,
			IsPointer:  isPointer,
			StructLitData: &lift.StructLiteralData{
				PkgPath:  typePkgPath,
				PkgName:  typePkgName,
				TypeName: typeName,
				Fields:   fieldValues,
			},
		}
		resolvedDeps[providerID] = dep
		depGraph[dep] = prereqs
		*allDeps = append(*allDeps, dep)
		return desiredVarName, dep, nil

	case *ast.SelectorExpr: // e.g., `somepkg.SomeConst` or `somepkg.SomeVar`
		// Treat as an inline expression.
		renderable, err := c.exprToString(n)
		return renderable, nil, err

	default:
		return "", nil, fmt.Errorf("unsupported AST expression type for dependency resolution: %T", expr)
	}
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
		return nil, fmt.Errorf("circular dependency detected in dependency graph")
	}

	return result, nil
}
