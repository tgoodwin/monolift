package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/pkg/compiler"
)

const progname = "monolift"

type options struct {
	dirname                 string
	outputDir               string
	dockerRegistry          string
	originalK8sManifestPath string // New field for the K8s manifest path
}

func rootCmd() *cobra.Command {
	opts := options{}

	root := &cobra.Command{
		Use:   progname,
		Short: "Monolith compiler for Kubernetes",
		Run: func(_ *cobra.Command, _ []string) {
			start(&opts)
		},
	}

	root.PersistentFlags().StringVarP(&opts.dirname, "dirname", "d", "", "go program directory to parse")
	root.PersistentFlags().StringVarP(&opts.outputDir, "output", "o", "output", "directory to create generated files")
	root.PersistentFlags().StringVarP(
		&opts.dockerRegistry,
		"docker-registry",
		"r",
		"localhost:5000",
		"location of docker registry to push to",
	)
	root.PersistentFlags().StringVarP(
		&opts.originalK8sManifestPath,
		"manifest",
		"m",
		"",
		"path to the original application's Kubernetes deployment manifest (for extracting env vars/args)",
	)
	root.AddCommand(liftCmd())
	return root
}

func start(opts *options) {
	if opts.dirname == "" {
		fmt.Printf("Error: --dirname is required\n")
		os.Exit(1)
	}
	c, err := compiler.New(opts.dirname) // Pass new option
	if err != nil {
		fmt.Printf("Error initializing compiler: %v\n", err)
		os.Exit(1)
	} // Pass new option
	if err := c.Compile(opts.outputDir, opts.dirname, opts.dockerRegistry, opts.originalK8sManifestPath); err != nil {
		fmt.Printf("Error during compilation: %v\n", err)
		os.Exit(1)
	}
}

type liftOptions struct {
	source            string
	target            string
	trace             string
	output            string
	serviceName       string
	hostImage         string
	extractedImage    string
	hostServiceName   string
	hostBuildPackage  string
	hostBinaryName    string
	hostPort          int
	hostReadinessPath string
	hostEnv           []string
	writeMonolithStub bool
	timeout           time.Duration
}

func liftCmd() *cobra.Command {
	opts := liftOptions{}
	cmd := &cobra.Command{
		Use:   "lift",
		Short: "Generate HTTP/JSON lift artifacts for an activation-path cut",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.source == "" {
				return fmt.Errorf("--source is required")
			}
			if opts.target == "" {
				return fmt.Errorf("--target is required")
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if opts.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, opts.timeout)
				defer cancel()
			}
			hostEnvVars, err := parseHostEnv(opts.hostEnv)
			if err != nil {
				return err
			}
			return codegen.RunLift(ctx, codegen.LiftOptions{
				Source:      opts.source,
				Target:      opts.target,
				Trace:       opts.trace,
				Output:      opts.output,
				ServiceName: opts.serviceName,
				Deploy: codegen.DeployOptions{
					HostImage:         opts.hostImage,
					ExtractedImage:    opts.extractedImage,
					HostServiceName:   opts.hostServiceName,
					HostBuildPackage:  opts.hostBuildPackage,
					HostBinaryName:    opts.hostBinaryName,
					HostPort:          opts.hostPort,
					HostReadinessPath: opts.hostReadinessPath,
					HostEnvVars:       hostEnvVars,
				},
				WriteMonolithStub: opts.writeMonolithStub,
			})
		},
	}
	cmd.Flags().StringVar(&opts.source, "source", "", "source module root")
	cmd.Flags().StringVar(&opts.target, "target", "", "target function source location as file:line")
	cmd.Flags().StringVar(&opts.trace, "trace", "", "optional activation trace JSON to pin the path")
	cmd.Flags().StringVar(&opts.output, "output", "", "directory for generated files")
	cmd.Flags().StringVar(&opts.serviceName, "service-name", "", "generated service name")
	cmd.Flags().StringVar(&opts.hostImage, "host-image", "", "container image for the patched host")
	cmd.Flags().StringVar(&opts.extractedImage, "extracted-image", "", "container image for the extracted service")
	cmd.Flags().StringVar(&opts.hostServiceName, "host-service-name", "", "Kubernetes service name for the patched host")
	cmd.Flags().StringVar(&opts.hostBuildPackage, "host-build-package", "", "Go package to build for the patched host Dockerfile")
	cmd.Flags().StringVar(&opts.hostBinaryName, "host-binary-name", "", "binary name for the patched host Dockerfile")
	cmd.Flags().IntVar(&opts.hostPort, "host-port", 0, "container port for the patched host")
	cmd.Flags().StringVar(&opts.hostReadinessPath, "host-readiness-path", "", "readiness probe path for the patched host")
	cmd.Flags().StringArrayVar(&opts.hostEnv, "host-env", nil, "host environment variable in KEY=VALUE form; repeatable")
	cmd.Flags().BoolVar(&opts.writeMonolithStub, "write-monolith-stub", false, "patch the monolith callsite to use the generated stub")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 120*time.Second, "activation-path analysis timeout")
	return cmd
}

func parseHostEnv(raw []string) ([]codegen.EnvVar, error) {
	env := make([]codegen.EnvVar, 0, len(raw))
	for _, item := range raw {
		name, value, ok := strings.Cut(item, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("--host-env must be KEY=VALUE, got %q", item)
		}
		env = append(env, codegen.EnvVar{Name: name, Value: value})
	}
	return env, nil
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
