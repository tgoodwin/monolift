package compiler

import (
	"fmt"
	"go/ast"
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

	for importPath, astPkg := range compiler.Packages {
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
										for _, lp := range compiler.LoadedPkgs {
											if lp.PkgPath == importPath { // importPath is from the outer loop
												currentLoadedPkg = lp
												break
											}
										}

										if currentLoadedPkg != nil && currentLoadedPkg.TypesInfo != nil {
											implementerNamedType, err := compiler.FindSingleImplementer(typeSpec.Name, currentLoadedPkg)
											if err != nil {
												fmt.Printf("      Error finding implementer for %s.%s: %v\n", importPath, typeSpec.Name.Name, err)
											} else if implementerNamedType != nil {
												fmt.Printf("      Interface %s.%s is implemented by struct %s.%s\n",
													importPath, typeSpec.Name.Name,
													implementerNamedType.Obj().Pkg().Path(), implementerNamedType.Obj().Name())

												// Prepare data for template generation
												serverStructName := strings.ToLower(typeSpec.Name.Name[:1]) + typeSpec.Name.Name[1:] + "Server"
												delegateFieldName := strings.ToLower(typeSpec.Name.Name[:1]) + typeSpec.Name.Name[1:] + "Delegate"

												// Extract methods from the interface for template generation
												var methodDataList []lift.MethodData
												ifaceObj := currentLoadedPkg.TypesInfo.Defs[typeSpec.Name]
												if ifaceObj == nil {
													fmt.Printf("      Could not find type object for interface %s\n", typeSpec.Name.Name)
												} else if ifaceTypeName, ok := ifaceObj.(*types.TypeName); !ok {
													fmt.Printf("      Object for %s is not a TypeName\n", typeSpec.Name.Name)
												} else if ifaceType, ok := ifaceTypeName.Type().Underlying().(*types.Interface); !ok {
													fmt.Printf("      Type for %s is not an Interface\n", typeSpec.Name.Name)
												} else {
													for i := 0; i < ifaceType.NumExplicitMethods(); i++ {
														method := ifaceType.ExplicitMethod(i)
														methodName := method.Name()
														handlerFuncName := "handle" + strings.ToUpper(methodName[:1]) + methodName[1:]
														httpRoute := "/" + strings.ToLower(methodName)

														methodDataList = append(methodDataList, lift.MethodData{
															Name:            methodName,
															HandlerFuncName: handlerFuncName,
															HTTPRoute:       httpRoute,
														})
													}
												}

												templateData := lift.ServerTemplateData{
													InterfacePackageAlias: currentLoadedPkg.Name, // Assumes currentLoadedPkg.Name is suitable as an alias
													InterfacePackagePath:  currentLoadedPkg.PkgPath,
													InterfaceTypeName:     typeSpec.Name.Name,
													ServerStructName:      serverStructName,
													DelegateFieldName:     delegateFieldName,
													Methods:               methodDataList,
													Imports:               make(map[string]string), // Initialize; will populate later
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

	return compiler, nil
}

// FindSingleImplementer takes an AST identifier for an interface name and its defining package,
// then searches through all loaded application packages to find a single struct type
// that implements this interface.
// If no implementers are found, it returns (nil, error).
// If multiple implementers are found, it returns (nil, error listing them).
// If exactly one implementer is found, it returns the *types.Named for the struct and nil error.
func (c *Compiler) FindSingleImplementer(ifaceNameIdent *ast.Ident, definingPkg *packages.Package) (*types.Named, error) {
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
