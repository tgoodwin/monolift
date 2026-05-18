package activation

// CutFeasibility classifies whether a candidate boundary can be implemented.
// FeasibleWithProxy is retained for backward compatibility but no longer
// produced by the analyzer (ADR-0028: streaming types at the cut point are
// Infeasible, signaling "cut deeper").
type CutFeasibility string

const (
	Feasible          CutFeasibility = "Feasible"
	FeasibleWithProxy CutFeasibility = "FeasibleWithProxy" // retained for JSON compat; not produced
	Infeasible        CutFeasibility = "Infeasible"
)

// BoundaryDataClass describes how hard the function boundary values are to
// move across a network boundary.
type BoundaryDataClass string

const (
	Trivial            BoundaryDataClass = "Trivial"
	Serializable       BoundaryDataClass = "Serializable"
	Reconstructible    BoundaryDataClass = "Reconstructible"
	ProxyRequired      BoundaryDataClass = "ProxyRequired"
	BoundaryInfeasible BoundaryDataClass = "BoundaryInfeasible"
)

// CallbackClass summarizes whether a boundary would require calls back into
// code above the cut.
type CallbackClass string

const (
	ZeroConfirmed CallbackClass = "ZeroConfirmed"
	ZeroEstimated CallbackClass = "ZeroEstimated"
	Low           CallbackClass = "Low"
	Moderate      CallbackClass = "Moderate"
	Many          CallbackClass = "Many"
)

// StateClass describes the state reconstruction burden at a candidate cut.
type StateClass string

const (
	Stateless             StateClass = "Stateless"
	ConfigOnly            StateClass = "ConfigOnly"
	ClientReconstructible StateClass = "ClientReconstructible"
	SharedState           StateClass = "SharedState"
)

// SurfaceClass approximates the remote API surface implied by a candidate cut.
type SurfaceClass string

const (
	Minimal   SurfaceClass = "Minimal"
	Small     SurfaceClass = "Small"
	Medium    SurfaceClass = "Medium"
	Large     SurfaceClass = "Large"
	VeryLarge SurfaceClass = "VeryLarge"
)

// ErrorSemClass describes how naturally a boundary can expose transport
// failure semantics.
type ErrorSemClass string

const (
	ErrorOK         ErrorSemClass = "ErrorOK"
	NeedsWrapper    ErrorSemClass = "NeedsWrapper"
	ErrorInfeasible ErrorSemClass = "ErrorInfeasible"
)

// AdapterClass describes whether the compiler can synthesize a local wrapper
// to normalize the boundary shape. It is orthogonal to BoundaryDataClass:
// BoundaryDataClass describes how hard boundary values are to serialize;
// AdapterClass describes whether a pattern-based adapter can bridge the gap.
// The two axes are never folded together (ADR-0028, ADR-0032).
type AdapterClass string

const (
	// DirectBoundary means the source signature is already serializable
	// or reconstructible enough for codegen — no adapter needed.
	DirectBoundary AdapterClass = "DirectBoundary"

	// AdapterPossible means the compiler can synthesize a local wrapper
	// plus normalized remote payloads.
	AdapterPossible AdapterClass = "AdapterPossible"

	// AdapterUnknown means a boundary might be adaptable, but the compiler
	// has not proved the required transforms.
	AdapterUnknown AdapterClass = "AdapterUnknown"

	// LiveProxyRequired means remote code would need to interact with a
	// host-owned live object. Refuse — this is a reason to look for another
	// cut, not a deployment mode (ADR-0028).
	LiveProxyRequired AdapterClass = "LiveProxyRequired"

	// AdapterImpossible means static proof fails or preserving semantics
	// would require changing the app.
	AdapterImpossible AdapterClass = "AdapterImpossible"
)

// EdgeAlignClass rates whether an activation edge naturally aligns with a
// network boundary.
type EdgeAlignClass string

const (
	Strong EdgeAlignClass = "Strong"
	Weak   EdgeAlignClass = "Weak"
	Anti   EdgeAlignClass = "Anti"
)
