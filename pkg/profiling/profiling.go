package profiling

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	pprofProfile "github.com/google/pprof/profile"

	"github.com/tgoodwin/monolift/pkg/profiling/internal/tree"
)

func intakeCPUProfileURL(path string, sampleSeconds int) (*pprofProfile.Profile, error) {

	// pprof cpu endpoint of form "http://localhost:6060/debug/pprof/profile"
	// Parse the URL
	parsedURL, err := url.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid URL: %v\n", err)
		os.Exit(1)
	}

	// Check if the path ends with "/profile"
	// Add sampleSeconds as a query parameter if it does
	if strings.HasSuffix(parsedURL.Path, "/profile") {
		q := parsedURL.Query()
		q.Set("seconds", fmt.Sprintf("%d", sampleSeconds))
		parsedURL.RawQuery = q.Encode()
		fmt.Printf("Fetching CPU profile for %d seconds...\n", sampleSeconds)
	} else {
		fmt.Printf("Fetching profile from %s...\n", parsedURL.String())
	}

	// Make HTTP request
	resp, err := http.Get(parsedURL.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to GET profile: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	profileData, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read profile data: %v\n", err)
		os.Exit(1)
	}

	// Parse it using pprof's profile parser
	profile, err := pprofProfile.ParseData(profileData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse profile: %v\n", err)
		os.Exit(1)
	}
	return profile, nil
}

// Intake file return profile object
func intakeProfileFile(path string) (*pprofProfile.Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open profile: %w", err)
	}
	defer f.Close()
	prof, err := pprofProfile.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}
	return prof, nil
}

func remove_string(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			// new string slice without the element
			slice := make([]string, len(s)-1)
			copy(slice, s[:i])       // Copy elements before the removed element
			copy(slice[i:], s[i+1:]) // Copy elements after the removed element
			return slice
		}
	}
	return s
}

func removeChildrenCosts(node *tree.FlameGraphNode) int64 {
	if node == nil {
		return 0
	}
	totalCost := node.Value
	for _, child := range node.Children {
		totalCost -= child.Value
	}
	return totalCost
}

// Returns root of flamegraph tree
func BuildTree(prof *pprofProfile.Profile) (*tree.FlameGraphNode, error) {
	return tree.BuildFlameGraphTree(prof, 0)
}

// FunctionNode is a struct that represents a function in the profile tree
// It contains the function name, the total value of the function (e.g. CPU time),
// and children nodes representing functions called by this function.
type FunctionNode struct {
	Name       string
	TotalValue int64
	SelfValue  int64 // Excludes the cost of child nodes

	Children []*FunctionNode
}

// ProfileUnit is a struct that holds a single profile
// It contains the profile data, flamegraph source root, and a reduced function tree if applicable.
// Methods:
// - ProportionOfFunctionFromTotal(funcName string) float64
// - FunctionCostWithoutChildren(funcName string) int64
// - FindTopNFunctionsWithoutChildCosts(n int) []FunctionNode
// - GetProfileSubsetCountSortedList(functionNode *FunctionNode, sortExcludingChildCost bool) []FunctionNode
// - GetProfileFunctionSubset(functions []string) *FunctionNode
type ProfileUnit struct {
	// Name of the profile
	Name string
	// Type of the profile (e.g. CPU, Memory)
	Type string
	// Total value of the profile (e.g. total CPU time)
	TotalValue int64

	// Profile raw data
	Profile *pprofProfile.Profile

	// Flamegraph tree representation
	FlamegraphSourceRoot *tree.FlameGraphNode

	// Reduced profile tree for CPU Profiles
	// functions given in the profile
	FunctionRootNode *FunctionNode
}

// ProfileInspector is a struct that holds multiple Profiles
// Methods:
// - InspectPprofFile(path []string) ProfileInspector
// - MergeProfiles(inspector ProfileInspector, profileNames []string) ProfileUnit
// - GetProfileFunctionSubset(profileName string, functions []string) *FunctionNode
type ProfileInspector struct {
	Profiles map[string]ProfileUnit
}

// TODO: Test
func (inspector *ProfileInspector) InspectPprofEndpoints(paths []string, sampleSeconds int) error {

	inspector.Profiles = make(map[string]ProfileUnit)

	for _, p := range paths {
		log.Printf("Inspecting profile endpoint: %s", p)

		prof, err := intakeCPUProfileURL(p, sampleSeconds)
		if err != nil {
			log.Fatalf("Error reading profile URL: %v", err)
			return err
		}
		root, err := BuildTree(prof)
		if err != nil {
			log.Fatalf("Error building profile tree: %v", err)
			return err
		}
		profileUnit := ProfileUnit{
			Name:                 p,
			Type:                 "cpu",      // Default type, can be changed later
			TotalValue:           root.Value, // Will be calculated later
			Profile:              prof,
			FlamegraphSourceRoot: root,
			FunctionRootNode:     nil, // Will be set later
		}

		inspector.Profiles[p] = profileUnit
	}
	return nil
}

// TODO: Test
func (inspector *ProfileInspector) InspectPprofFiles(paths []string) error {

	inspector.Profiles = make(map[string]ProfileUnit)

	for _, p := range paths {
		log.Printf("Inspecting profile file: %s", p)

		prof, err := intakeProfileFile(p)
		if err != nil {
			log.Fatalf("Error reading profile file: %v", err)
			return err
		}
		root, err := BuildTree(prof)
		if err != nil {
			log.Fatalf("Error building profile tree: %v", err)
			return err
		}
		profileUnit := ProfileUnit{
			Name:                 p,
			Type:                 "cpu",      // Default type, can be changed later
			TotalValue:           root.Value, // Will be calculated later
			Profile:              prof,
			FlamegraphSourceRoot: root,
			FunctionRootNode:     nil, // Will be set later
		}

		inspector.Profiles[p] = profileUnit
	}
	return nil
}

// TODO: Test
func (inspector *ProfileInspector) MergeProfiles(profileNames []string) ProfileUnit {

	concatenatedName := ""
	for _, name := range profileNames {
		if _, exists := inspector.Profiles[name]; !exists {
			log.Printf("Profile %s not found in inspector", name)
			return ProfileUnit{} // Return empty ProfileUnit if profile not found
		}

		concatenatedName += name + "_"
	}

	rawProfiles := make([]*pprofProfile.Profile, 0, len(profileNames))
	for _, name := range profileNames {
		profile := inspector.Profiles[name]
		rawProfiles = append(rawProfiles, profile.Profile)
	}

	rawMergedProfile, err := pprofProfile.Merge(rawProfiles)
	if err != nil {
		log.Fatalf("Error merging Profiles: %v", err)
	}

	mergedFlameGraphRoot, err := BuildTree(rawMergedProfile)
	if err != nil {
		log.Fatalf("Error building merged profile tree: %v", err)
	}

	// Create a new ProfileUnit to hold the merged profile
	mergedProfile := ProfileUnit{
		Name:                 concatenatedName,
		Type:                 "cpu",
		TotalValue:           mergedFlameGraphRoot.ObjectCount,
		Profile:              rawMergedProfile,
		FlamegraphSourceRoot: mergedFlameGraphRoot,
		FunctionRootNode:     nil, // Will be set later
	}
	inspector.Profiles[concatenatedName] = mergedProfile

	return mergedProfile
}

// TODO: Test
func (inspector *ProfileInspector) GetProfileFunctionSubset(profileName string, functions []string) *FunctionNode {
	// Check if the profile exists in the inspector
	if inspector.Profiles[profileName].Profile == nil {
		log.Printf("Profile %s not found in inspector", profileName)
		return nil // Return nil if profile not found
	}
	// Get the profile unit from the inspector
	profileUnit := inspector.Profiles[profileName]
	return profileUnit.GetProfileFunctionSubset(functions)
}

func (profile *ProfileUnit) GetFunctionListMatchingPrefixList(prefixList []string) []string {
	var functions []string
	visited := make(map[string]bool) // to avoid duplicates

	// Recursive traversal function
	var traverse func(node *tree.FlameGraphNode)

	traverse = func(node *tree.FlameGraphNode) {
		for _, prefix := range prefixList {
			if strings.HasPrefix(node.Name, prefix) {
				if !visited[node.Name] {
					functions = append(functions, node.Name)
					visited[node.Name] = true
				}
				break // no need to check other prefixes for this node
			}
		}
		for _, child := range node.Children {
			traverse(child)
		}
	}

	traverse(profile.FlamegraphSourceRoot)
	return functions
}

// TODO: Test
func (profile *ProfileUnit) GetProfileFunctionSubset(functions []string) *FunctionNode {

	// Include root by default
	functions = append(functions, "root")

	//DFS to construct a tree of only relevant functions
	// This will traverse the profile tree and build a new tree
	// containing only the functions specified in the functions list
	var dfs func(node *tree.FlameGraphNode, functions []string) []*FunctionNode

	// Subset of the profile tree containing only relevant functions
	dfs = func(node *tree.FlameGraphNode, functions []string) []*FunctionNode {
		if node == nil {
			return nil
		}

		for _, funcName := range functions {
			// if node in functions, begin building a new FunctionNode
			if node.Name == funcName {

				functionNode := new(FunctionNode)
				functionNode.Name = node.Name
				functionNode.TotalValue = node.Value
				functionNode.Children = make([]*FunctionNode, 0)
				tempSelfValue := node.Value //Temporary self value to calculate self cost

				// Found a function, remove it from the list when searching for children
				newFunctions := remove_string(functions, funcName)

				// Add children nodes
				for _, child := range node.Children {
					childNodes := dfs(child, newFunctions)

					if childNodes != nil {
						functionNode.Children = append(functionNode.Children, childNodes...)

						for _, childNode := range childNodes {
							// Subtract the child's total value from the parent's temp self value
							tempSelfValue -= childNode.TotalValue
						}
					}
				}
				functionNode.SelfValue = tempSelfValue

				return []*FunctionNode{functionNode}
			} else {
				continue
			}
		}
		// If not in functions, continue DFS
		// This will skip adding this node to the functionNode
		// and continue searching for relevant functions
		childSet := make([]*FunctionNode, 0)
		for _, child := range node.Children {
			childNode := dfs(child, functions)

			if childNode != nil {
				childSet = append(childSet, childNode...)
			}
		}
		if len(childSet) > 0 {
			// If children were found, return them
			return childSet
		}
		if len(node.Children) == 0 {
			childSet = nil
			// If no children, return nil
			return nil
		}
		return nil
	}

	rootFunctionNode := dfs(profile.FlamegraphSourceRoot, functions)
	profile.FunctionRootNode = rootFunctionNode[0]
	return profile.FunctionRootNode
}

// TODO: Test
func (profile *ProfileUnit) GetProfileSubsetCountSortedList(functionNode *FunctionNode, sortExcludingChildCost bool) []FunctionNode {
	var subsetList []FunctionNode
	var dfs func(node *FunctionNode)

	dfs = func(node *FunctionNode) {
		if node == nil {
			return
		}
		subsetList = append(subsetList, *node)
		for _, child := range node.Children {
			dfs(child)
		}
	}
	dfs(functionNode)

	sort.Slice(subsetList, func(i, j int) bool {
		if sortExcludingChildCost {
			return subsetList[i].SelfValue > subsetList[j].SelfValue
		} else {
			return subsetList[i].TotalValue > subsetList[j].TotalValue
		}
	})

	return subsetList
}

// Find smallest value in a map of string to int64
func findSmallestValueInMap(m map[string]int64) (string, int64) {
	smallestKey := ""
	smallestValue := int64(0)

	for key, value := range m {
		if smallestKey == "" || value < smallestValue {
			smallestKey = key
			smallestValue = value
		}
	}
	return smallestKey, smallestValue
}

func round(x float64) int {
	return int(math.Floor(x + 0.5))
}

// Heuristics:
// Entrypoint service should always have at least one instance
// If a service has a very small allocation value, it is a candidate for removal and it's value is allocated to the frontend service
func (profile *ProfileUnit) SimpleAllocationDistributionByFunction(serviceList []string, num_instances int, entrypointService string) map[string]float64 {

	newFunctionList := profile.GetFunctionListMatchingPrefixList(serviceList)
	functionRootNode := profile.GetProfileFunctionSubset(newFunctionList)
	// Get Subset Funcs
	subsetFuncs := profile.GetProfileSubsetCountSortedList(functionRootNode, true)

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
	minimalInstanceWeight := totalServiceCost / int64(num_instances)
	// print cost by service
	fmt.Printf("\nCost by Service:\n")
	fmt.Printf("Total Service Cost: %d\n", totalServiceCost)
	fmt.Printf("Minimal Instance Weight: %d\n", minimalInstanceWeight)

	valueByService := make(map[string]int64)
	// Calculate initial costs, identify inital allocation values
	for service, cost := range costByService {
		proportion := (float64(cost) / float64(totalServiceCost))
		fmt.Printf("%s: %d (%.2f%%)\n", service, cost, proportion*100)

		allocationValue := float64(proportion) * float64(num_instances)
		fmt.Printf("Initial Allocation Value: %.2f /%d\n", allocationValue, num_instances)
		valueByService[service] = cost
	}

	// If the frontend service has less than minimalInstanceWeight, steel minimalInstanceWeight from largest service
	if valueByService[entrypointService] < int64(minimalInstanceWeight) {
		fmt.Printf("Entrypoint service %s has less than minimalInstanceWeight of %d, stealing from largest service\n", entrypointService, minimalInstanceWeight)

		// find the largest service
		largestService := ""
		largestServiceValue := int64(0)
		for service, value := range costByService {
			if service == entrypointService {
				continue // Skip the frontend service
			}
			if value > largestServiceValue {
				largestService = service
				largestServiceValue = value
			}
		}

		// subtract minimalInstanceWeight from largest service
		if largestService != "" && largestServiceValue > minimalInstanceWeight {
			valueByService[largestService] -= int64(minimalInstanceWeight)
			valueByService[entrypointService] += int64(minimalInstanceWeight)
			fmt.Printf("Stealing %d from %s to %s\n", minimalInstanceWeight, largestService, entrypointService)
		} else {
			fmt.Printf("No suitable service found to steal from, keeping current allocation.\n")
		}
	}

	// Loop until all services have at least minimalInstanceWeight
	for {
		//Find Smallest Service below minimalInstanceWeight
		smallestService, smallestServiceValue := findSmallestValueInMap(valueByService)

		if smallestService != "" && smallestServiceValue < minimalInstanceWeight {
			fmt.Printf("Service %s has allocation %d below minimal allocation threshold of %d\n", smallestService, valueByService[smallestService], minimalInstanceWeight)
			// Remove the smallest service from the allocation map
			delete(valueByService, smallestService)
			// Add its value to the next smallest service, concatenating the name
			nextSmallestService, nextSmallestValue := findSmallestValueInMap(valueByService)
			newName := fmt.Sprintf("%s|%s", smallestService, nextSmallestService)
			fmt.Printf("Combining %s and %s into %s\n", smallestService, nextSmallestService, newName)
			// Update the allocation map
			delete(valueByService, nextSmallestService)
			valueByService[newName] = nextSmallestValue + smallestServiceValue
		}

		if smallestService == "" || valueByService[smallestService] >= int64(minimalInstanceWeight) { // All services have at least minimalInstanceWeight, break the loop
			break
		}
	}

	allocationByService := make(map[string]float64)
	// Calculate final allocation values
	for service, value := range valueByService {
		proportion := float64(value) / float64(totalServiceCost)
		allocationByService[service] = proportion * float64(num_instances)
	}

	fmt.Printf("\nFinal Allocation by Service:\n")
	for service, allocation := range allocationByService {
		fmt.Printf("%s: %.2f (%.2f%%)\n", service, allocation, (allocation/float64(num_instances))*100)
		// Round up to nearest multiple of 8
		rounded_allocation := round(allocation)
		fmt.Printf("Rounded Allocation: %d\n", rounded_allocation)
	}

	return allocationByService
}

func (profile *ProfileUnit) ProportionOfRootFromName(funcName string) float64 {
	root := profile.FlamegraphSourceRoot
	targetNode := tree.SearchNodeByName(root, funcName)
	if targetNode == nil || root.Value == 0 {
		return 0.0
	}
	return float64(targetNode.Value) / float64(root.Value)
}

func (profile *ProfileUnit) FunctionCostWithoutChildren(targetName string) int64 {
	root := profile.FlamegraphSourceRoot
	parent := tree.SearchNodeByName(root, targetName)
	cost := parent.Value

	for _, child := range parent.Children {
		cost -= child.Value
	}
	return cost
}

// TODO
func (profile *ProfileUnit) FindTopNFunction(n int, excludeChildValue bool) []FunctionNode {
	root := profile.FlamegraphSourceRoot
	var stats []FunctionNode
	var dfs func(node *tree.FlameGraphNode)
	dfs = func(node *tree.FlameGraphNode) {
		if node == nil {
			return
		}
		totalNodeCount := node.Value
		costWithoutChildren := removeChildrenCosts(node)
		stats = append(stats, FunctionNode{Name: node.Name, TotalValue: totalNodeCount, SelfValue: costWithoutChildren})
		for _, child := range node.Children {
			dfs(child)
		}
	}
	dfs(root)

	if excludeChildValue {
		// Sort by SelfValue if excluding child costs
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].SelfValue > stats[j].SelfValue
		})
	} else {
		// Sort by TotalValue if including child costs
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].TotalValue > stats[j].TotalValue
		})
	}
	if len(stats) > n {
		return stats[:n]
	}
	return stats
}

func main() {

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s  profile1.pb.gz profile2.pb.gz ...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	pathList := flag.Args()
	profileInspector := &ProfileInspector{}
	profileInspector.InspectPprofFiles(pathList)
	profileUnit := profileInspector.Profiles[pathList[0]]

	funcs := profileUnit.FindTopNFunction(10, false)
	fmt.Printf("Top Functions: \n")
	for i, f := range funcs {
		fmt.Printf("%d. %s: %d\n", i+1, f.Name, f.TotalValue)
	}

	funcsChildless := profileUnit.FindTopNFunction(10, true)
	fmt.Printf("Top Functions: \n")
	for i, f := range funcsChildless {
		fmt.Printf("%d. %s: %d\n", i+1, f.Name, f.SelfValue)
	}

	functionRootNode := profileUnit.GetProfileFunctionSubset([]string{"github.com/harlow/go-micro-services/services/search/proto.(*searchClient).Nearby", "github.com/harlow/go-micro-services/services/reservation/proto.(*reservationClient).CheckAvailability", "github.com/harlow/go-micro-services/services/frontend.(*Server).recommendHandler"})

	subsetFuncs := profileUnit.GetProfileSubsetCountSortedList(functionRootNode, false)
	fmt.Printf("Reduced profile tree with only relevant functions:\n")
	for i, f := range subsetFuncs {
		fmt.Printf("%d. %s: %d\n", i+1, f.Name, f.TotalValue)
	}

}
