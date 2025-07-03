package metrics

import (
	"fmt"
	"os"
	"strconv"
)

const (
	// EnvK8sCPURequest is the default environment variable for the CPU request.
	EnvK8sCPURequest = "K8S_CPU_REQUEST"
	// EnvK8sCPULimit is the default environment variable for the CPU limit.
	EnvK8sCPULimit = "K8S_CPU_LIMIT"
	// EnvK8sMemRequest is the default environment variable for the memory request.
	EnvK8sMemRequest = "K8S_MEM_REQUEST"
	// EnvK8sMemLimit is the default environment variable for the memory limit.
	EnvK8sMemLimit = "K8S_MEM_LIMIT"
)

// CPURequestFromEnv reads the CPU request from the environment.
// It expects the value to be exposed via the Downward API in millicores.
// The value is returned in cores (e.g., 1.5).
func CPURequestFromEnv() (float64, error) {
	// The value from the Downward API is in millicores (e.g., "500" for "500m").
	milliCores, err := readUintFromEnv(EnvK8sCPURequest)
	if err != nil {
		return 0, err
	}
	// Convert millicores to cores for internal use.
	return float64(milliCores) / 1000.0, nil
}

// CPULimitFromEnv reads the CPU limit from the environment.
// It expects the value to be exposed via the Downward API in millicores.
// The value is returned in cores (e.g., 1.5).
func CPULimitFromEnv() (float64, error) {
	milliCores, err := readUintFromEnv(EnvK8sCPULimit)
	if err != nil {
		return 0, err
	}
	return float64(milliCores) / 1000.0, nil
}

// MemoryRequestFromEnv reads the memory request from the environment.
// It expects the value to be exposed via the Downward API.
// The value is returned in bytes.
func MemoryRequestFromEnv() (uint64, error) {
	return readUintFromEnv(EnvK8sMemRequest)
}

// MemoryLimitFromEnv reads the memory limit from the environment.
// It expects the value to be exposed via the Downward API.
// The value is returned in bytes.
func MemoryLimitFromEnv() (uint64, error) {
	return readUintFromEnv(EnvK8sMemLimit)
}

// readUintFromEnv is a helper to read and parse a uint64 from an env var.
func readUintFromEnv(varName string) (uint64, error) {
	valStr := os.Getenv(varName)
	if valStr == "" {
		return 0, fmt.Errorf("environment variable %s is not set or is empty", varName)
	}

	val, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse uint from env var %s (value: '%s'): %w", varName, valStr, err)
	}

	return val, nil
}
