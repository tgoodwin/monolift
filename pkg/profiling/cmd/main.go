package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/tgoodwin/monolift/pkg/profiling"
)

func extractRPS(r *regexp.Regexp, path string) int {
	base := filepath.Base(path)
	matches := r.FindStringSubmatch(base)
	if len(matches) == 2 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			return val
		}
	}
	return 0 // fallback value
}

func loadFunctions(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var funcs []string
	if err := json.Unmarshal(data, &funcs); err != nil {
		return nil, err
	}
	return funcs, nil
}

func main() {

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s <profile_dir> <service_list_file>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(flag.Args()) != 2 {
		flag.Usage()
		os.Exit(1)
	}
	profileDir := flag.Arg(0)
	serviceListFile := flag.Arg(1)

	serviceList, err := loadFunctions(serviceListFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading function list from %s: %v\n", serviceListFile, err)
		os.Exit(1)
	}

	dir := []string{profileDir}
	// Collect all pprof files from the provided directory
	// Get all file paths in the directory
	pathList := make([]string, 0)
	for _, path := range dir {
		files, err := os.ReadDir(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading directory %s: %v\n", path, err)
			os.Exit(1)
		}
		for _, file := range files {
			if file.IsDir() {
				continue // Skip directories
			}
			filePath := fmt.Sprintf("%s/%s", path, file.Name())
			pathList = append(pathList, filePath)
		}
	}

	// Sort pathList according to _rps suffix
	// This is a simple sort based on the assumption that the file names end with "_rps"
	// You may need to adjust this if your naming convention is different
	// Regex to extract RPS value, e.g., rps-100
	r := regexp.MustCompile(`-(\d+)\.out$`)

	sort.Slice(pathList, func(i, j int) bool {
		rpsI := extractRPS(r, pathList[i])
		rpsJ := extractRPS(r, pathList[j])
		return rpsI < rpsJ
	})

	profileInspector := &profiling.ProfileInspector{}
	profileInspector.InspectPprofFiles(pathList)

	for _, path := range pathList {
		fmt.Printf("\nProfile: %s", path)

		profileUnit := profileInspector.Profiles[path]
		profileUnit.SimpleAllocationDistributionByFunction(serviceList, 8, serviceList[0]) // Assuming the first service is the frontend service

	}

}
