package pragma

import (
	"errors"
	"fmt"
	"strconv"
)

type SignalType string

const (
	CPUTrigger SignalType = "CPU"
	MemTrigger SignalType = "MEM"
	// Add other signal types here in the future
)

var signalTypes = map[SignalType]struct{}{
	CPUTrigger: {},
	MemTrigger: {},
}

func getSignalType(s string) (SignalType, bool) {
	_, ok := signalTypes[SignalType(s)]
	return SignalType(s), ok
}

type Pragma struct {
	SignalType SignalType
	Threshold  float64
}

func ParsePragma(attrs map[string]string) (*Pragma, error) {
	// If there are no attributes, it's a simple extraction without a delegate.
	if len(attrs) == 0 {
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

	thresholdVal, ok := attrs["threshold"]
	if !ok {
		return nil, errors.New("pragma is missing required 'threshold' attribute for delegate")
	}
	threshold, err := strconv.ParseFloat(thresholdVal, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid 'threshold' value: %q, must be a float: %w", thresholdVal, err)
	}
	// For CPU/MEM, the threshold is a percentage (e.g., 0.5 for 50%).
	if threshold <= 0 || threshold > 1.0 {
		return nil, fmt.Errorf("invalid 'threshold' value: %f, must be between 0.0 and 1.0", threshold)
	}

	return &Pragma{
		SignalType: signalType,
		Threshold:  threshold,
	}, nil
}
