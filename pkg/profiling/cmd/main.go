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
	"strings"

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
		profileUnit := profileInspector.Profiles[path]

		newFunctionList := profileUnit.GetFunctionListMatchingPrefixList(serviceList)

		functionRootNode := profileUnit.GetProfileFunctionSubset(newFunctionList)
		// Get Subste Funcs
		subsetFuncs := profileUnit.GetProfileSubsetCountSortedList(functionRootNode, true)

		// // Get total value of all functions in the profile
		// totalValue := functionRootNode.TotalValue

		// fmt.Printf("\nTotal Value of all functions in profile %s: %d\n", path, totalValue)
		// fmt.Printf("Self Cost:\n")
		// for i, f := range subsetFuncs {
		// 	if f.Name == functionRootNode.Name {
		// 		// Skip the root function, we will add its self value later
		// 		continue
		// 	}

		// 	fmt.Printf("%d. %s: %d\n", i+1, f.Name, f.SelfValue)
		// 	// get proportion of self value to total value
		// 	proportion := (float64(f.SelfValue) / float64(totalValue))
		// 	fmt.Printf("   Proportion: %.2f%%\n", proportion*100)

		// 	// Calculate possible speedup according to Amdahl's Law
		// 	// speedup := 1 / (1 - (float64(proportion)) + (float64(proportion) / 8))
		// 	// fmt.Printf("   Possible Speedup: %.2f\n", speedup)

		// }

		costByService := make(map[string]int64)
		for _, f := range subsetFuncs {
			if f.Name == functionRootNode.Name {
				// Skip the root function, we will add its self value later
				continue
			}
			// Check if the function name is in the service list
			for _, service := range serviceList {
				if strings.HasPrefix(f.Name, service) {
					costByService[service] += f.SelfValue
					break // No need to check other services if we found a match
				}
			}
		}
		totalServiceCost := int64(0)
		for _, cost := range costByService {
			totalServiceCost += cost
		}

		num_instances := 8
		allocationByService := make(map[string]float64)
		fmt.Printf("\nProfile: %s\n", path)
		// print cost by service
		fmt.Printf("\nCost by Service:\n")
		fmt.Printf("Total Service Cost: %d\n", totalServiceCost)
		for service, cost := range costByService {
			proportion := (float64(cost) / float64(totalServiceCost))
			fmt.Printf("%s: %d (%.2f%%)\n", service, cost, proportion*100)

			allocationValue := float64(proportion) * float64(num_instances)
			fmt.Printf("Allocation Value: %.2f /%d\n", allocationValue, num_instances)
			allocationByService[service] = allocationValue

			// Calculate possible speedup according to Amdahl's Law
			// speedup := 1 / (1 - (float64(proportion)) + (float64(proportion) / float64(allocationValue)))
			// fmt.Printf("   Possible Speedup: %.2f\n", speedup)
		}

	}

}
