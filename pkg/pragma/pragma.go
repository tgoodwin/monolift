package pragma

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// SignalType defines the type of signal for a pragma.
type SignalType string

const (
	// CPUTrigger represents a CPU usage signal.
	CPUTrigger SignalType = "CPU"
	// MemTrigger represents a memory usage signal.
	MemTrigger SignalType = "MEM"
	// IPSTrigger represents an invocations per second signal.
	IPSTrigger SignalType = "IPS"
)

var signalTypes = map[SignalType]struct{}{
	CPUTrigger: {},
	MemTrigger: {},
	IPSTrigger: {},
}

func getSignalType(s string) (SignalType, bool) {
	st := SignalType(strings.ToUpper(s))
	_, ok := signalTypes[st]
	return st, ok
}

// Pragma represents a parsed monolift directive.
type Pragma struct {
	SignalType SignalType
	Threshold  float64
	// Add other pragma fields as needed
}

// ParsePragma parses a map of attributes into a Pragma struct.
// Example pragma: trigger=CPU threshold=0.8
// Example pragma: trigger=IPS threshold=100
func ParsePragma(attrs map[string]string) (*Pragma, error) {
	// If there are no attributes, it's a simple extraction without a delegate.
	if len(attrs) == 0 {
		// Return a pragma with no signal type, indicating simple extraction.
		return &Pragma{}, nil
	}

	triggerVal, ok := attrs["trigger"]
	if !ok {
		return nil, errors.New("pragma is missing required 'trigger' attribute for delegate")
	}
	signalType, ok := getSignalType(triggerVal)
	if !ok {
		return nil, fmt.Errorf("invalid or unsupported signal type for 'trigger': %q", triggerVal)
	}

	var threshold float64
	var err error

	switch signalType {
	case IPSTrigger:
		valueVal, ok := attrs["threshold"]
		if !ok {
			return nil, errors.New("pragma with trigger=IPS is missing required 'threshold' attribute")
		}
		threshold, err = strconv.ParseFloat(valueVal, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid 'value' for IPS trigger: %q, must be a float: %w", valueVal, err)
		}
	case CPUTrigger, MemTrigger:
		thresholdVal, ok := attrs["threshold"]
		if !ok {
			return nil, fmt.Errorf("pragma with trigger=%s is missing required 'threshold' attribute", signalType)
		}
		threshold, err = strconv.ParseFloat(thresholdVal, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid 'threshold' value: %q, must be a float: %w", thresholdVal, err)
		}
		// For CPU/MEM, the threshold is a percentage (e.g., 0.5 for 50%).
		if threshold <= 0 || threshold > 1.0 {
			return nil, fmt.Errorf("invalid 'threshold' value for %s: %f, must be between 0.0 and 1.0", signalType, threshold)
		}
	}

	return &Pragma{
		SignalType: signalType,
		Threshold:  threshold,
	}, nil
}
