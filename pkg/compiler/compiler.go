package compiler

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Compiler holds the parsed ASTs for the application's Go packages.
// It discovers packages within the application's module starting from a root directory.
type Compiler struct {
	Fset *token.FileSet
	// Packages are keyed by their import path (e.g., "my/app/main", "my/app/utils").
	// Each *ast.Package contains the files for that specific package.
	Packages map[string]*ast.Package
}

// New parses all Go packages belonging to the application's module, starting from appRootPath.
// It excludes third-party dependencies.
func New(appRootPath string) (*Compiler, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedModule,
		Dir:  appRootPath, // directory from which to run the go list-like commands.
		Fset: fset,        // for associating AST nodes with the correct file positions.
	}

	// Load all packages within the module found at appRootPath and its subdirectories.
	loadedPkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load packages from %s: %w", appRootPath, err)
	}

	appPackages := make(map[string]*ast.Package)
	var errors []string
	for _, pkg := range loadedPkgs {
		for _, pkgErr := range pkg.Errors {
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
		Fset:     fset,
		Packages: appPackages,
	}

	targetPackageImportPath := ""
	// Determine the correct import path for userservice.
	// This might be "dapr-apps/socialnet/monolith/userservice" or something based on your go.mod module path + /userservice
	// For example, if your go.mod is "my/project", it might be "my/project/demo/monolith/userservice"
	// You might need to list `compiler.Packages` keys to find the exact one.
	// For now, let's iterate to find it:
	for pkgPath := range appPackages {
		if strings.HasSuffix(pkgPath, "socialgraph") { // Adjust this heuristic if needed
			targetPackageImportPath = pkgPath
			break
		}
	}

	if pkgAst, ok := appPackages[targetPackageImportPath]; ok {
		for filePath, fileAst := range pkgAst.Files {
			if strings.HasSuffix(filePath, "service.go") { // Assuming the file is named service.go
				fmt.Printf("\nDEBUG: Inspecting AST for %s in package %s\n", filePath, targetPackageImportPath)
				for _, decl := range fileAst.Decls {
					if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
						for _, spec := range genDecl.Specs {
							if typeSpec, okSpec := spec.(*ast.TypeSpec); okSpec {
								if typeSpec.Name.Name == "Service" {
									fmt.Printf("  DEBUG: Found TypeSpec for 'Service':\n")
									if typeSpec.Doc != nil {
										fmt.Printf("    DEBUG: typeSpec.Doc is NOT nil. Text:\n%s\n", typeSpec.Doc.Text())
									} else {
										fmt.Printf("    DEBUG: typeSpec.Doc IS nil.\n")
									}
									if genDecl.Doc != nil {
										fmt.Printf("    DEBUG: genDecl.Doc is NOT nil. Text:\n%s\n", genDecl.Doc.Text())
									} else {
										fmt.Printf("    DEBUG: genDecl.Doc IS nil.\n")
									}
									// Also check ast.File.Comments for unassociated comments
									// This is more involved, but good to keep in mind.
								}
							}
						}
					}
				}
			}
		}
	} else if targetPackageImportPath != "" {
		fmt.Printf("\nDEBUG: Target package %s not found in parsed packages.\n", targetPackageImportPath)
	} else {
		fmt.Printf("\nDEBUG: Could not determine target package for userservice.\n")
	}
	// ---- END DETAILED DEBUGGING ----

	// Diagnostic printing
	for importPath, astPkg := range compiler.Packages {
		fmt.Printf("Found package (import path: %s, name: %s)\n", importPath, astPkg.Name)
		for filePath, fileAst := range astPkg.Files {
			fmt.Printf("  File: %s\n", filePath)
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
				case *ast.TypeSpec:
					// This case might still be useful for TypeSpecs not within a GenDecl
					// or if GenDecl.Doc is not the one we want.
					// However, for the current issue, we handle TypeSpecs within GenDecls below.
					// Consider if you need standalone TypeSpec pragma checking.
					// For now, we'll rely on the GenDecl logic for types.
					pass := true // Placeholder to avoid empty switch case if FuncDecl is removed
					_ = pass
				case *ast.GenDecl:
					// Check for TYPE declarations (like interfaces, structs)
					if node.Tok == token.TYPE {
						for _, spec := range node.Specs {
							if typeSpec, ok := spec.(*ast.TypeSpec); ok {
								if IsInterface(typeSpec) {
									fmt.Println("	Found interface:", typeSpec.Name.Name)
									var docCommentGroup *ast.CommentGroup
									if typeSpec.Doc != nil {
										docCommentGroup = typeSpec.Doc
									} else if node.Doc != nil { // node here is the ast.GenDecl
										docCommentGroup = node.Doc
									}

									pragmas := GetPragmasFromCommentGroup(docCommentGroup)
									if len(pragmas) > 0 {
										fmt.Printf("    Interface %s has pragmas:\n", typeSpec.Name.Name)
										for _, pragma := range pragmas {
											fmt.Printf("      %s\n", pragma.Raw)
										}
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
