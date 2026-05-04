package main

import "github.com/spf13/cobra"

func main() {
	cmd := &cobra.Command{
		Use:  "demo",
		RunE: run,
	}
	_ = cmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	target()
	return nil
}

func target() {}
