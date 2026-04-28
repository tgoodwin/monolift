package entrypath

import "fmt"

type FunctionIndexMode string
type BoundaryDiscoveryMode string

const (
	FunctionIndexModeAll         FunctionIndexMode = "all"
	FunctionIndexModeReversePath FunctionIndexMode = "reverse-path"
	// FunctionIndexModeHTTPSinks is the legacy CLI spelling. New boundary
	// work treats HTTP as one BoundaryPredicate rather than the whole design.
	FunctionIndexModeHTTPSinks    FunctionIndexMode = "http-sinks"
	FunctionIndexModeTargeted     FunctionIndexMode = "targeted"
	FunctionIndexModeBridge       FunctionIndexMode = "bridge"
	FunctionIndexModeOracleBridge FunctionIndexMode = "oracle-bridge"
)

const (
	BoundaryDiscoveryModeAll      BoundaryDiscoveryMode = "all"
	BoundaryDiscoveryModeFrontier BoundaryDiscoveryMode = "frontier"
)

func ParseFunctionIndexMode(value string) (FunctionIndexMode, error) {
	mode := FunctionIndexMode(value)
	if mode == "" {
		mode = FunctionIndexModeAll
	}
	if !mode.Valid() {
		return "", fmt.Errorf("invalid function index mode %q", value)
	}
	return mode, nil
}

func ParseBoundaryDiscoveryMode(value string) (BoundaryDiscoveryMode, error) {
	mode := BoundaryDiscoveryMode(value)
	if mode == "" {
		mode = BoundaryDiscoveryModeAll
	}
	if !mode.Valid() {
		return "", fmt.Errorf("invalid boundary discovery mode %q", value)
	}
	return mode, nil
}

func (mode FunctionIndexMode) Valid() bool {
	switch mode {
	case "", FunctionIndexModeAll, FunctionIndexModeReversePath, FunctionIndexModeHTTPSinks, FunctionIndexModeTargeted, FunctionIndexModeBridge, FunctionIndexModeOracleBridge:
		return true
	default:
		return false
	}
}

func (mode BoundaryDiscoveryMode) Valid() bool {
	switch mode {
	case "", BoundaryDiscoveryModeAll, BoundaryDiscoveryModeFrontier:
		return true
	default:
		return false
	}
}

func (mode FunctionIndexMode) OrDefault() FunctionIndexMode {
	if mode == "" {
		return FunctionIndexModeAll
	}
	return mode
}

func (mode BoundaryDiscoveryMode) OrDefault() BoundaryDiscoveryMode {
	if mode == "" {
		return BoundaryDiscoveryModeAll
	}
	return mode
}
