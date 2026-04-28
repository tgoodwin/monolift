package entrypath

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type ProbeResult struct {
	RegionRoots         []TraceNode        `json:"regionRoots"`
	ExternalSurfaces    []ExternalSurface  `json:"externalSurfaces"`
	RegistrationSites   []RegistrationSite `json:"registrationSites"`
	WrapperChains       []WrapperChain     `json:"wrapperChains"`
	RegionTouchpoints   []RegionTouchpoint `json:"regionTouchpoints"`
	BootStartCandidates []TraceNode        `json:"bootStartCandidates"`
	OracleTrace         *OracleTrace       `json:"oracleTrace,omitempty"`
	Diagnostics         []Diagnostic       `json:"diagnostics"`
	Stats               Stats              `json:"stats"`
}

type TraceNode struct {
	ID       string                  `json:"id"`
	Label    string                  `json:"label"`
	Identity reportv2.SymbolIdentity `json:"identity"`
	Position SourcePosition          `json:"position"`
}

type SourcePosition struct {
	Filename string `json:"filename"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

type ExternalSurface struct {
	Node      TraceNode `json:"node"`
	EdgeKind  string    `json:"edgeKind"`
	Evidence  []string  `json:"evidence"`
	RegionIDs []string  `json:"regionIds"`
}

// RegistrationSite records a ValueSink where a function value reaches an
// InvocationBoundary. The JSON name is stable for existing probe consumers.
type RegistrationSite struct {
	Node                TraceNode `json:"node"`
	EdgeKind            string    `json:"edgeKind"`
	StaticParameterType string    `json:"staticParameterType,omitempty"`
	SinkKind            string    `json:"sinkKind,omitempty"`
	Handler             TraceNode `json:"handler"`
	RegionIDs           []string  `json:"regionIds"`
}

type WrapperChain struct {
	RegionRoot       TraceNode     `json:"regionRoot"`
	ExternalSurface  TraceNode     `json:"externalSurface"`
	Links            []WrapperLink `json:"links"`
	RegistrationSite TraceNode     `json:"registrationSite"`
}

type WrapperLink struct {
	From     TraceNode      `json:"from"`
	To       TraceNode      `json:"to"`
	EdgeKind string         `json:"edgeKind"`
	Site     SourcePosition `json:"site"`
}

type RegionTouchpoint struct {
	RegionRoot TraceNode `json:"regionRoot"`
	Touchpoint TraceNode `json:"touchpoint"`
	EdgeKind   string    `json:"edgeKind"`
}

type Diagnostic struct {
	Kind        string         `json:"kind"`
	Reason      string         `json:"reason,omitempty"`
	Function    string         `json:"function,omitempty"`
	Instruction string         `json:"instruction,omitempty"`
	Position    SourcePosition `json:"position"`
}

type Stats struct {
	FunctionCount              int                    `json:"functionCount"`
	StaticEdgeCount            int                    `json:"staticEdgeCount"`
	DynamicEdgeCount           int                    `json:"dynamicEdgeCount"`
	UnresolvedDynamicSiteCount int                    `json:"unresolvedDynamicSiteCount"`
	CallgraphAlgorithm         string                 `json:"callgraphAlgorithm"`
	WallClockMillis            int64                  `json:"wallClockMillis"`
	PeakRSSBytes               uint64                 `json:"peakRSSBytes"`
	PhaseTimings               []PhaseTiming          `json:"phaseTimings,omitempty"`
	FunctionRefIndex           FunctionRefIndexStats  `json:"functionRefIndex"`
	FunctionIndexSeeds         FunctionIndexSeedStats `json:"functionIndexSeeds"`
	BoundaryDiscovery          BoundaryDiscoveryStats `json:"boundaryDiscovery"`
	BridgeDiscovery            BridgeDiscoveryStats   `json:"bridgeDiscovery"`
	RootResolution             RootResolutionStats    `json:"rootResolution"`
}

type BoundaryDiscoveryStats struct {
	Mode                    string             `json:"mode,omitempty"`
	ReverseFrontierOwners   int                `json:"reverseFrontierOwners"`
	ReverseOwners           int                `json:"reverseOwners"`
	AdjacentExpansionOwners int                `json:"adjacentExpansionOwners"`
	CandidateOwnerCount     int                `json:"candidateOwnerCount"`
	BoundaryCandidateOwners int                `json:"boundaryCandidateOwners"`
	CandidatePackageCount   int                `json:"candidatePackageCount"`
	BoundarySeedOwners      int                `json:"boundarySeedOwners"`
	BoundaryEvidenceCount   int                `json:"boundaryEvidenceCount"`
	BoundaryEvidence        int                `json:"boundaryEvidence"`
	SeedSetOwners           int                `json:"seedSetOwners"`
	FinalIndexedOwners      int                `json:"finalIndexedOwners"`
	BudgetStops             []BudgetStopReason `json:"budgetStops,omitempty"`
	StopReasons             []string           `json:"stopReasons,omitempty"`
}

type BudgetStopReason struct {
	Budget string `json:"budget"`
	Reason string `json:"reason"`
}

type BridgeDiscoveryStats struct {
	TouchpointCount              int                          `json:"touchpointCount"`
	StartCandidateCount          int                          `json:"startCandidateCount"`
	SelectedStartCount           int                          `json:"selectedStartCount"`
	SkippedStartCount            int                          `json:"skippedStartCount"`
	SkipReasons                  []BridgeSkipReason           `json:"skipReasons,omitempty"`
	StartPackageCount            int                          `json:"startPackageCount"`
	ScannedPackageCount          int                          `json:"scannedPackageCount"`
	ScannedPackageFunctions      int                          `json:"scannedPackageFunctions"`
	ScannedInstructions          int                          `json:"scannedInstructions"`
	BridgeOwnerCount             int                          `json:"bridgeOwnerCount"`
	BridgeBoundaryCandidateCount int                          `json:"bridgeBoundaryCandidateCount"`
	BridgeBoundaryOwnerCount     int                          `json:"bridgeBoundaryOwnerCount"`
	BridgeSeedOwnerCount         int                          `json:"bridgeSeedOwnerCount"`
	IndexedBridgeOwnerCount      int                          `json:"indexedBridgeOwnerCount"`
	IndexPriorityClassCounts     []BridgeIndexClassCount      `json:"indexPriorityClassCounts,omitempty"`
	IndexSkipReasonCounts        []BridgeIndexSkipReasonCount `json:"indexSkipReasonCounts,omitempty"`
	DuplicateOwnerSuppressions   int                          `json:"duplicateOwnerSuppressions"`
	BudgetStops                  []BudgetStopReason           `json:"budgetStops,omitempty"`
	StopReasons                  []string                     `json:"stopReasons,omitempty"`
	Coverage                     BridgeCoverage               `json:"coverage,omitempty"`
}

type BridgeSkipReason struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type BridgeCoverage struct {
	Starts        []BridgeStartCoverage        `json:"starts,omitempty"`
	Packages      []BridgePackageCoverage      `json:"packages,omitempty"`
	OracleTargets []BridgeOracleTargetCoverage `json:"oracleTargets,omitempty"`
	RefMatchAudit []BridgeRefMatchAudit        `json:"refMatchAudit,omitempty"`
	IndexOwners   []BridgeIndexOwnerDiagnostic `json:"indexOwners,omitempty"`
}

type BridgeIndexClassCount struct {
	PriorityClass string `json:"priorityClass"`
	Count         int    `json:"count"`
	Indexed       int    `json:"indexed"`
	Skipped       int    `json:"skipped"`
}

type BridgeIndexSkipReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type BridgeIndexOwnerDiagnostic struct {
	Function          string                    `json:"function,omitempty"`
	PackagePath       string                    `json:"packagePath,omitempty"`
	ObjectName        string                    `json:"objectName,omitempty"`
	SeedReasons       []string                  `json:"seedReasons,omitempty"`
	PriorityClass     string                    `json:"priorityClass,omitempty"`
	PriorityRank      int                       `json:"priorityRank"`
	PriorityInputs    BridgeIndexPriorityInputs `json:"priorityInputs"`
	IndexOrder        int                       `json:"indexOrder"`
	Indexed           bool                      `json:"indexed"`
	SkipReason        string                    `json:"skipReason,omitempty"`
	BudgetResponsible string                    `json:"budgetResponsible,omitempty"`
}

type BridgeIndexPriorityInputs struct {
	BridgeSeed                bool     `json:"bridgeSeed"`
	BoundarySeed              bool     `json:"boundarySeed"`
	BoundaryEvidenceCount     int      `json:"boundaryEvidenceCount"`
	SelectedTouchpointPackage bool     `json:"selectedTouchpointPackage"`
	DirectTouchpointRefs      int      `json:"directTouchpointRefs"`
	SeedReasons               []string `json:"seedReasons,omitempty"`
}

type BridgeStartCoverage struct {
	Function               string   `json:"function,omitempty"`
	PackagePath            string   `json:"packagePath,omitempty"`
	ObjectName             string   `json:"objectName,omitempty"`
	SelectedPackage        string   `json:"selectedPackage,omitempty"`
	Scheduled              bool     `json:"scheduled"`
	Scanned                bool     `json:"scanned"`
	Completed              bool     `json:"completed"`
	ScannedFunctionCount   int      `json:"scannedFunctionCount"`
	InstructionCount       int      `json:"instructionCount"`
	BridgeOwnersAdmitted   int      `json:"bridgeOwnersAdmitted"`
	BoundaryOwnersAdmitted int      `json:"boundaryOwnersAdmitted"`
	StopReasons            []string `json:"stopReasons,omitempty"`
	SkipCauses             []string `json:"skipCauses,omitempty"`
}

type BridgePackageCoverage struct {
	PackagePath            string   `json:"packagePath"`
	SelectedStartCount     int      `json:"selectedStartCount"`
	SelectedStarts         []string `json:"selectedStarts,omitempty"`
	Scheduled              bool     `json:"scheduled"`
	Scanned                bool     `json:"scanned"`
	Completed              bool     `json:"completed"`
	ScannedFunctionCount   int      `json:"scannedFunctionCount"`
	InstructionCount       int      `json:"instructionCount"`
	BridgeOwnersAdmitted   int      `json:"bridgeOwnersAdmitted"`
	BoundaryOwnersAdmitted int      `json:"boundaryOwnersAdmitted"`
	StopReasons            []string `json:"stopReasons,omitempty"`
	SkipCauses             []string `json:"skipCauses,omitempty"`
}

type BridgeOracleTargetCoverage struct {
	ID                         string   `json:"id,omitempty"`
	Function                   string   `json:"function,omitempty"`
	PackagePath                string   `json:"packagePath,omitempty"`
	ObjectName                 string   `json:"objectName,omitempty"`
	PackageSelected            bool     `json:"packageSelected"`
	PackageScheduled           bool     `json:"packageScheduled"`
	PackageScanned             bool     `json:"packageScanned"`
	PackageCompleted           bool     `json:"packageCompleted"`
	ScanningStoppedBeforeOwner bool     `json:"scanningStoppedBeforeOwner"`
	OwnerScanned               bool     `json:"ownerScanned"`
	RefMatcherInspected        bool     `json:"refMatcherInspected"`
	ProducedBridgeSeed         bool     `json:"producedBridgeSeed"`
	FunctionRefIndexed         bool     `json:"functionRefIndexed"`
	BoundaryPredicatesRan      bool     `json:"boundaryPredicatesRan"`
	BoundaryPredicateRejected  bool     `json:"boundaryPredicateRejected"`
	BoundaryEvidence           []string `json:"boundaryEvidence,omitempty"`
	StopReasons                []string `json:"stopReasons,omitempty"`
	SkipCauses                 []string `json:"skipCauses,omitempty"`
}

type BridgeRefMatchAudit struct {
	ID                               string               `json:"id,omitempty"`
	Function                         string               `json:"function,omitempty"`
	PackagePath                      string               `json:"packagePath,omitempty"`
	ObjectName                       string               `json:"objectName,omitempty"`
	Audited                          bool                 `json:"audited"`
	BridgeScanned                    bool                 `json:"bridgeScanned"`
	RefMatcherInspected              bool                 `json:"refMatcherInspected"`
	Counts                           BridgeRefMatchCounts `json:"counts"`
	Refs                             []BridgeRefMatchRef  `json:"refs,omitempty"`
	StaticCalleesReceivingTouchpoint []string             `json:"staticCalleesReceivingTouchpoint,omitempty"`
	BoundaryEvidence                 []string             `json:"boundaryEvidence,omitempty"`
	SeedReasons                      []string             `json:"seedReasons,omitempty"`
	ProducedBridgeSeed               bool                 `json:"producedBridgeSeed"`
	SeedResult                       string               `json:"seedResult,omitempty"`
}

type BridgeRefMatchCounts struct {
	DirectTouchpointRefs int `json:"directTouchpointRefs"`
	CallArgs             int `json:"callArgs"`
	Stores               int `json:"stores"`
	Returns              int `json:"returns"`
	Closures             int `json:"closures"`
	DirectInvokes        int `json:"directInvokes"`
}

type BridgeRefMatchRef struct {
	Kind         string         `json:"kind"`
	Touchpoint   string         `json:"touchpoint,omitempty"`
	Instruction  string         `json:"instruction,omitempty"`
	StaticCallee string         `json:"staticCallee,omitempty"`
	Position     SourcePosition `json:"position"`
}

type FunctionIndexSeedStats struct {
	OwnerCount                     int `json:"ownerCount"`
	ReversePathOwners              int `json:"reversePathOwners"`
	BoundarySeedOwners             int `json:"boundarySeedOwners"`
	BoundaryEvidenceCount          int `json:"boundaryEvidenceCount"`
	HTTPSinkOwners                 int `json:"httpSinkOwners"`
	OnDemandExpansionOwners        int `json:"onDemandExpansionOwners"`
	BridgeOwners                   int `json:"bridgeOwners"`
	OracleBridgeOwners             int `json:"oracleBridgeOwners"`
	RejectedNonHTTPInterfaceOwners int `json:"rejectedNonHTTPInterfaceOwners"`
}

type RootResolutionStats struct {
	FunctionsInspected int   `json:"functionsInspected"`
	MatchedSpecs       int   `json:"matchedSpecs"`
	FastPathHits       int   `json:"fastPathHits"`
	FallbackHits       int   `json:"fallbackHits"`
	ElapsedMillis      int64 `json:"elapsedMillis"`
	RSSDeltaBytes      int64 `json:"rssDeltaBytes"`
}

type FunctionRefIndexStats struct {
	ScannedFunctions          int    `json:"scannedFunctions"`
	ScannedBlocks             int    `json:"scannedBlocks"`
	ScannedInstructions       int    `json:"scannedInstructions"`
	DiscoveredFunctionSources int    `json:"discoveredFunctionSources"`
	ClosureSources            int    `json:"closureSources"`
	OperandRefs               int    `json:"operandRefs"`
	CallArgRefs               int    `json:"callArgRefs"`
	StoreRefs                 int    `json:"storeRefs"`
	ReturnRefs                int    `json:"returnRefs"`
	SkippedFunctions          int    `json:"skippedFunctions"`
	ElapsedMillis             int64  `json:"elapsedMillis"`
	PeakRSSBytes              uint64 `json:"peakRSSBytes"`
}

type PhaseTiming struct {
	Name            string `json:"name"`
	WallClockMillis int64  `json:"wallClockMillis"`
	PeakRSSBytes    uint64 `json:"peakRSSBytes"`
}

type PhaseEvent struct {
	Name                string
	Status              string
	WallClockMillis     int64
	PeakRSSBytes        uint64
	ScannedFunctions    int
	ScannedBlocks       int
	ScannedInstructions int
	CurrentPackagePath  string
}

// ProbeOptions controls EntryPath diagnostics. BoundaryPredicate and SeedSet
// concepts are represented here as bounded function-index modes and seed knobs
// to preserve the existing report shape while frontier diagnostics evolve.
type ProbeOptions struct {
	PhaseObserver                         func(PhaseEvent)
	FunctionIndexMode                     FunctionIndexMode
	BoundaryDiscoveryMode                 BoundaryDiscoveryMode
	BoundaryFrontierMaxOwners             int
	BoundaryFrontierMaxReverseOwners      int
	BoundaryFrontierMaxAdjacentOwners     int
	BoundaryFrontierMaxBoundaryCandidates int
	BoundaryFrontierDepth                 int
	BoundaryFrontierMaxPackages           int
	BoundaryFrontierMaxDuration           time.Duration
	FunctionRefIndexProgressInterval      int
	FunctionRefIndexBudget                time.Duration
	FunctionRefIndexMaxFunctions          int
	TargetedExpansionMaxFunctions         int
	TargetedExpansionMaxDepth             int
	TargetedExpansionMaxDuration          time.Duration
	TargetedExpansionMaxQueue             int
	BridgeMaxStarts                       int
	BridgeMaxPackages                     int
	BridgeMaxPackageFunctions             int
	BridgeMaxOwners                       int
	BridgeMaxBoundaryOwners               int
	BridgeMaxInstructions                 int
	BridgeMaxDuration                     time.Duration
	OracleSpec                            OracleSpec
	OracleBridgeMaxPackageFunctions       int
	OracleBridgeMaxOwners                 int
	OracleBridgeMaxDuration               time.Duration
}

const (
	EdgeStaticCall                 = "EdgeStaticCall"
	EdgeDynamicInterface           = "EdgeDynamicInterface"
	EdgeMethodCall                 = "EdgeMethodCall"
	EdgeFunctionValueArg           = "EdgeFunctionValueArg"
	EdgeFunctionValueStoredField   = "EdgeFunctionValueStoredField"
	EdgeFunctionValueStoredGlobal  = "EdgeFunctionValueStoredGlobal"
	EdgeFunctionValueReturned      = "EdgeFunctionValueReturned"
	EdgeClosureCapture             = "EdgeClosureCapture"
	EdgeGoroutineLaunch            = "EdgeGoroutineLaunch"
	EdgeFunctionValueStoredElement = "EdgeFunctionValueStoredElement"
	EdgeDirectInvoke               = "EdgeDirectInvoke"
)
