package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tgoodwin/monolift/pkg/compiler"
)

func main() {
	outputDir := flag.String("o", "/tmp/monolift-sprint-0013", "output directory")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Println("Usage: extract-report [-o output_dir] <source_dir>...")
		os.Exit(1)
	}

	sources := flag.Args()
	for i, s := range sources {
		abs, err := filepath.Abs(s)
		if err == nil {
			sources[i] = abs
		}
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	pragmas, diagnostics, err := compiler.Parse(sources)
	if err != nil {
		fmt.Printf("Error parsing sources: %v\n", err)
		os.Exit(1)
	}

	// Filter diagnostics to show warnings/errors during parse
	for _, d := range diagnostics {
		fmt.Printf("Parse Diagnostic: [%s] %s at %s\n", d.Code, d.Message, d.Span)
	}

	for _, p := range pragmas {
		fmt.Printf("Processing pragma: %s (Decl: %s)\n", p.Name, p.DeclName)
		
		// Create a single-pragma request for this root
		report, extractDiagnostics, err := compiler.Extract(sources, []*compiler.Pragma{p})
		if err != nil {
			fmt.Printf("Error extracting %s: %v\n", p.Name, err)
			continue
		}

		filename := fmt.Sprintf("report-%s.json", p.Name)
		outputPath := filepath.Join(*outputDir, filename)
		
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling report %s: %v\n", p.Name, err)
			continue
		}

		if _, err := os.ReadFile(outputPath); err == nil {
			fmt.Printf("Warning: Overwriting existing report at %s\n", outputPath)
		}

		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			fmt.Printf("Error writing report %s: %v\n", p.Name, err)
			continue
		}
		
		fmt.Printf("Wrote report to %s\n", outputPath)
		
		if len(extractDiagnostics) > 0 {
			diagPath := outputPath + ".diagnostics.json"
			diagData, _ := json.MarshalIndent(extractDiagnostics, "", "  ")
			_ = os.WriteFile(diagPath, diagData, 0644)
			fmt.Printf("Wrote %d diagnostics to %s\n", len(extractDiagnostics), diagPath)
		}
	}
}
