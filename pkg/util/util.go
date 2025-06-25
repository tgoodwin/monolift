package util

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func GenerateImports(filePath string) error {
	err := exec.Command("goimports", "-w", filePath).Run()
	if err != nil {
		return fmt.Errorf("could not run goimports: %w", err)
	}
	return nil
}

func InitGoMod(name, outputDir string) error {
	initCmd := exec.Command("go", "mod", "init", name)
	initCmd.Dir = outputDir
	if err := initCmd.Run(); err != nil {
		return fmt.Errorf("could not run go mod init: %w", err)
	}
	replaceCmd := exec.Command("go", "mod", "edit", "-replace=github.com/tgoodwin/monolift=../../")
	replaceCmd.Dir = outputDir
	if err := replaceCmd.Run(); err != nil {
		return fmt.Errorf("could not run go mod edit: %w", err)
	}
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = outputDir
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("could not run go mod tidy: %w", err)
	}
	return nil
}

// determineImportAlias decides whether an explicit alias is needed for an import.
// If the package's declared name is the same as the last component of its import path,
// no explicit alias is needed, and an empty string is returned. Otherwise, the package name is returned.
func DetermineImportAlias(pkgPath, pkgName string) string {
	if pkgName == filepath.Base(pkgPath) {
		return "" // No explicit alias needed, Go will use the package name by default
	}
	return pkgName // Use the package name as an explicit alias
}
