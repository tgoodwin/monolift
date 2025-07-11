package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
	if err := root.MarkPersistentFlagRequired("dirname"); err != nil {
		panic(err)
	}

	return root
}

func start(opts *options) {
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

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
