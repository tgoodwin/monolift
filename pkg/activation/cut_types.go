package activation

// CutFeasibility classifies whether a candidate boundary can be implemented
// directly, with a proxy, or not at all.
type CutFeasibility string

const (
	Feasible          CutFeasibility = "Feasible"
	FeasibleWithProxy CutFeasibility = "FeasibleWithProxy"
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

// EdgeAlignClass rates whether an activation edge naturally aligns with a
// network boundary.
type EdgeAlignClass string

const (
	Strong EdgeAlignClass = "Strong"
	Weak   EdgeAlignClass = "Weak"
	Anti   EdgeAlignClass = "Anti"
)
