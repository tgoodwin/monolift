package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	pprofProfile "github.com/google/pprof/profile"

	"monolift/profiling/internal/tree"
)

// Intake file return profile object
func intakeProfile(path string) (*pprofProfile.Profile, error) {
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

	// Reduced profile tree for CPU profiles
	// functions given in the profile
	FunctionRootNode *FunctionNode
}

// ProfileInspector is a struct that holds multiple profiles
// Methods:
// - InspectPprofFile(path []string) ProfileInspector
// - MergeProfiles(inspector ProfileInspector, profileNames []string) ProfileUnit
// - GetProfileFunctionSubset(profileName string, functions []string) *FunctionNode
type ProfileInspector struct {
	profiles map[string]ProfileUnit
}

// TODO: Test
func InspectPprofFiles(paths []string) ProfileInspector {

	profileInspector := ProfileInspector{
		profiles: make(map[string]ProfileUnit),
	}

	for _, p := range paths {
		log.Printf("Inspecting profile file: %s", p)

		prof, err := intakeProfile(p)
		if err != nil {
			log.Fatalf("Error reading profile file: %v", err)
		}
		root, err := BuildTree(prof)
		if err != nil {
			log.Fatalf("Error building profile tree: %v", err)
		}
		profileUnit := ProfileUnit{
			Name:                 p,
			Type:                 "cpu",      // Default type, can be changed later
			TotalValue:           root.Value, // Will be calculated later
			Profile:              prof,
			FlamegraphSourceRoot: root,
			FunctionRootNode:     nil, // Will be set later
		}

		profileInspector.profiles[p] = profileUnit
	}

	return profileInspector
}

// TODO: Test
func (inspector ProfileInspector) MergeProfiles(profileNames []string) ProfileUnit {

	concatenatedName := ""
	for _, name := range profileNames {
		if _, exists := inspector.profiles[name]; !exists {
			log.Printf("Profile %s not found in inspector", name)
			return ProfileUnit{} // Return empty ProfileUnit if profile not found
		}

		concatenatedName += name + "_"
	}

	rawProfiles := make([]*pprofProfile.Profile, 0, len(profileNames))
	for _, name := range profileNames {
		profile := inspector.profiles[name]
		rawProfiles = append(rawProfiles, profile.Profile)
	}

	rawMergedProfile, err := pprofProfile.Merge(rawProfiles)
	if err != nil {
		log.Fatalf("Error merging profiles: %v", err)
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
	inspector.profiles[concatenatedName] = mergedProfile

	return mergedProfile
}

// TODO: Test
func (inspector ProfileInspector) GetProfileFunctionSubset(profileName string, functions []string) *FunctionNode {
	// Check if the profile exists in the inspector
	if inspector.profiles[profileName].Profile == nil {
		log.Printf("Profile %s not found in inspector", profileName)
		return nil // Return nil if profile not found
	}
	// Get the profile unit from the inspector
	profileUnit := inspector.profiles[profileName]
	return profileUnit.GetProfileFunctionSubset(functions)
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
	profileInspector := InspectPprofFiles(pathList)
	profileUnit := profileInspector.profiles[pathList[0]]

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
