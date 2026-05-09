package activation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

type goListPackage struct {
	ImportPath string   `json:"ImportPath"`
	Name       string   `json:"Name"`
	Imports    []string `json:"Imports"`
}

// ReverseImportScope returns the set of package import paths that transitively
// import the package containing targetFile. It runs two lightweight go list
// passes (no type-checking) and builds a reverse-import graph via BFS.
//
// The returned set always includes the target package itself. Command packages
// (Name == "main") are included only if they transitively import the target.
func ReverseImportScope(dir, targetFile string, env []string) ([]string, error) {
	targetDir := filepath.Dir(targetFile)
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(dir, targetDir)
	}

	targetPkg, err := resolvePackagePath(targetDir, env)
	if err != nil {
		return nil, fmt.Errorf("resolve target package: %w", err)
	}

	allPkgs, err := listAllPackages(dir, env)
	if err != nil {
		return nil, fmt.Errorf("list module packages: %w", err)
	}

	reverse := make(map[string][]string)
	for _, pkg := range allPkgs {
		for _, imp := range pkg.Imports {
			reverse[imp] = append(reverse[imp], pkg.ImportPath)
		}
	}

	visited := make(map[string]bool)
	queue := []string{targetPkg}
	visited[targetPkg] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, importer := range reverse[cur] {
			if !visited[importer] {
				visited[importer] = true
				queue = append(queue, importer)
			}
		}
	}

	var scoped []string
	for _, pkg := range allPkgs {
		if visited[pkg.ImportPath] {
			scoped = append(scoped, pkg.ImportPath)
		}
	}
	sort.Strings(scoped)

	if len(scoped) == 0 {
		return []string{targetPkg}, nil
	}
	return scoped, nil
}

func resolvePackagePath(dir string, env []string) (string, error) {
	cmd := exec.Command("go", "list", "-json", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list -json in %s: %w", dir, err)
	}
	var pkg goListPackage
	if err := json.Unmarshal(out, &pkg); err != nil {
		return "", fmt.Errorf("parse go list output: %w", err)
	}
	return pkg.ImportPath, nil
}

func listAllPackages(dir string, env []string) ([]goListPackage, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -json ./... in %s: %w", dir, err)
	}
	var pkgs []goListPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var pkg goListPackage
		if err := dec.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("decode package: %w", err)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}
