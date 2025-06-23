package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tgoodwin/monolift/pkg/compiler"
)

const progname = "monolift"

type options struct {
	dirname        string
	outputDir      string
	dockerRegistry string
}

func rootCmd() *cobra.Command {
	opts := options{
		dirname: "foobar",
	}

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
		"docker.io/tlg2132",
		"location of docker registry to push to",
	)
	if err := root.MarkPersistentFlagRequired("dirname"); err != nil {
		panic(err)
	}

	return root
}

func start(opts *options) {
	c, err := compiler.New(opts.dirname)
	if err != nil {
		fmt.Println(err)
	}
	if err := c.Compile(); err != nil {
		panic(err)
	}
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
