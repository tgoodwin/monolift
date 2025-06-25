package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	_ "embed"
)

//go:embed embeds/Dockerfile
var dockerfile string

type goBuilder struct {
	goEnv []string
}

func newGoBuilder() (*goBuilder, error) {
	home := os.Getenv("HOME")
	goEnv := []string{
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH=arm64",
		fmt.Sprintf("GOPATH=%s/go", home),
		fmt.Sprintf("HOME=%s", home),
	}
	return &goBuilder{
		goEnv: goEnv,
	}, nil
}

func (self *goBuilder) build(outputDir, dockerRegistry string, names []string) error {
	for _, name := range names {
		workingDir := fmt.Sprintf("%s/%s", outputDir, name)

		//nolint:gosec // this is fine dot jpeg
		var buildCmd *exec.Cmd
		// The entrypoint is built from its package root, while services have a main.go
		if name == "entrypoint" {
			buildCmd = exec.Command("go", "build", "-trimpath", "-o", "main", ".")
		} else {
			// specifying main.go assumes that there are no other files in the main package
			buildCmd = exec.Command("go", "build", "-trimpath", "-o", "main", "main.go")
		}
		buildCmd.Dir = workingDir
		buildCmd.Env = self.goEnv
		buildCmd.Stderr = os.Stderr
		fmt.Printf("  Running %v\n", buildCmd)

		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("could not run go build for %s: %w", name, err)
		}

		f, err := os.Create(fmt.Sprintf("%s/Dockerfile", workingDir))
		if err != nil {
			return fmt.Errorf("could not create Dockerfile for %s: %w", name, err)
		}
		defer f.Close()

		fmt.Fprint(f, dockerfile)

		// TODO add some prefix to the docker image name that identifies the application the code was extracted from
		dockerPath := strings.ToLower(fmt.Sprintf("%s/%s:latest", dockerRegistry, name))
		dockerBuildCmd := exec.Command("docker", "build", ".", "-t", dockerPath)
		dockerBuildCmd.Dir = workingDir
		dockerBuildCmd.Stderr = os.Stderr
		fmt.Printf("  Running %v\n", dockerBuildCmd)

		if err := dockerBuildCmd.Run(); err != nil {
			return fmt.Errorf("could not run docker build for %s: %w", name, err)
		}

		dockerPushCmd := exec.Command("docker", "push", dockerPath)
		dockerPushCmd.Dir = workingDir
		dockerPushCmd.Stderr = os.Stderr
		fmt.Printf("  Running %v\n", dockerPushCmd)

		if err := dockerPushCmd.Run(); err != nil {
			return fmt.Errorf("could not run docker push for %s: %w", name, err)
		}
	}

	return nil
}
