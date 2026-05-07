package main

import (
	"context"
	"fmt"
	"os"
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
			return codegen.RunLift(ctx, codegen.LiftOptions{
				Source:            opts.source,
				Target:            opts.target,
				Trace:             opts.trace,
				Output:            opts.output,
				ServiceName:       opts.serviceName,
				WriteMonolithStub: opts.writeMonolithStub,
			})
		},
	}
	cmd.Flags().StringVar(&opts.source, "source", "", "source module root")
	cmd.Flags().StringVar(&opts.target, "target", "", "target function source location as file:line")
	cmd.Flags().StringVar(&opts.trace, "trace", "", "optional activation trace JSON to pin the path")
	cmd.Flags().StringVar(&opts.output, "output", "", "directory for generated files")
	cmd.Flags().StringVar(&opts.serviceName, "service-name", "", "generated service name")
	cmd.Flags().BoolVar(&opts.writeMonolithStub, "write-monolith-stub", false, "patch the monolith callsite to use the generated stub")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 120*time.Second, "activation-path analysis timeout")
	return cmd
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
