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

	// Diagnostic printing
	for importPath, astPkg := range compiler.Packages {
		fmt.Printf("Found package (import path: %s, name: %s)\n", importPath, astPkg.Name)
		for filePath := range astPkg.Files {
			fmt.Printf("  File: %s\n", filePath)
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
