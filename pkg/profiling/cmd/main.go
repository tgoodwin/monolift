package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tgoodwin/monolift/pkg/profiling"
)

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
	profileInspector := &profiling.ProfileInspector{}
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

	// // Compose Post
	functionRootNode := profileUnit.GetProfileFunctionSubset([]string{
		"github.com/tgoodwin/monolift/demo/monolith/frontend.(*APIHandlers).SaveHandler",
		"github.com/tgoodwin/monolift/demo/monolith/postservice.(*service).SavePost",
		"github.com/tgoodwin/monolift/demo/monolith/timelineservice.(*service).UpdateTimeline",
		""},
	)

	// Mixed trace
	// functionRootNode := profileUnit.GetProfileFunctionSubset([]string{
	// 	"github.com/tgoodwin/monolift/demo/monolith/postservice.(*service).SavePost",
	// 	"github.com/tgoodwin/monolift/demo/monolith/timelineservice.(*service).UpdateTimeline",
	// 	"github.com/tgoodwin/monolift/demo/monolith/postservice.(*service).ReadPosts",
	// 	"github.com/tgoodwin/monolift/demo/monolith/timelineservice.(*service).ReadTimeline",
	// 	""},
	// )

	subsetFuncs := profileUnit.GetProfileSubsetCountSortedList(functionRootNode, false)
	fmt.Printf("Reduced profile tree with only relevant functions:\n")
	for i, f := range subsetFuncs {
		fmt.Printf("%d. %s: %d\n", i+1, f.Name, f.TotalValue)
	}

	subsetFuncs = profileUnit.GetProfileSubsetCountSortedList(functionRootNode, true)
	fmt.Printf("Reduced profile tree with only relevant functions:\n")
	for i, f := range subsetFuncs {
		fmt.Printf("%d. %s: %d\n", i+1, f.Name, f.SelfValue)
	}

}
