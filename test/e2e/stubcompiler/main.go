package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	target := flag.String("target", "", "target fixture name")
	output := flag.String("output", "", "output directory")
	flag.Parse()

	if *target == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: stubcompiler --target=<name> --output=<dir>")
		os.Exit(2)
	}

	fixtureDir := filepath.Join("test", "e2e", "stubcompiler", "fixtures", *target)
	if _, err := os.Stat(fixtureDir); err != nil {
		fixtureDir = filepath.Join(repoRoot(), "test", "e2e", "stubcompiler", "fixtures", *target)
	}
	if err := copyTree(fixtureDir, *output); err != nil {
		fmt.Fprintf(os.Stderr, "stubcompiler target %s: %v\n", *target, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "stubcompiler emitted %s to %s\n", *target, *output)
}

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
	}
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		return copyFile(path, out)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
