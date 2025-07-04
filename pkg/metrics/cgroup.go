package metrics

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CgroupReader provides methods to read CPU and memory metrics from a cgroup.
// It is designed for a cgroup v2 filesystem.
type CgroupReader struct {
	// cgroupPath is the absolute path to the process's own cgroup directory.
	// e.g., /sys/fs/cgroup/user.slice/user-1000.slice/session-1.scope
	cgroupPath string
}

// NewCgroupReader initializes a new reader by finding the cgroup paths for the
// current process. It assumes a cgroup v2 hierarchy.
func NewCgroupReader() (*CgroupReader, error) {
	path, err := findCgroupPath()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cgroup reader: %w", err)
	}
	return &CgroupReader{cgroupPath: path}, nil
}

// CPUUsage calculates the CPU usage over a given sample duration.
// It blocks for the duration of the sample.
// The result is the number of CPU cores being used (e.g., 1.5 means 1.5 CPU
// cores worth of time was consumed).
func (r *CgroupReader) CPUUsage(sampleDuration time.Duration) (float64, error) {
	cpuStatPath := filepath.Join(r.cgroupPath, "cpu.stat")

	usage1, err := readCPUStat(cpuStatPath)
	if err != nil {
		return 0, fmt.Errorf("could not read initial cpu.stat: %w", err)
	}
	t1 := time.Now()

	time.Sleep(sampleDuration)

	usage2, err := readCPUStat(cpuStatPath)
	if err != nil {
		return 0, fmt.Errorf("could not read final cpu.stat: %w", err)
	}
	t2 := time.Now()

	cpuDelta := usage2 - usage1 // in microseconds
	timeDelta := t2.Sub(t1).Microseconds()

	if timeDelta == 0 {
		return 0, nil
	}

	usage := float64(cpuDelta) / float64(timeDelta)
	return usage, nil
}

// MemoryUsage returns the current memory consumption in bytes.
func (r *CgroupReader) MemoryUsage() (uint64, error) {
	memCurrentPath := filepath.Join(r.cgroupPath, "memory.current")
	usage, err := readUintFromFile(memCurrentPath)
	if err != nil {
		return 0, fmt.Errorf("could not read memory.current: %w", err)
	}
	return usage, nil
}

// MemoryLimit returns the memory limit in bytes.
// If there is no limit, it returns 0 and hasLimit will be false.
func (r *CgroupReader) MemoryLimit() (limit uint64, hasLimit bool, err error) {
	memMaxPath := filepath.Join(r.cgroupPath, "memory.max")
	content, err := os.ReadFile(memMaxPath)
	if err != nil {
		return 0, false, fmt.Errorf("could not read memory.max: %w", err)
	}

	strContent := strings.TrimSpace(string(content))
	if strContent == "max" {
		return 0, false, nil
	}

	limit, err = strconv.ParseUint(strContent, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("could not parse memory limit value '%s': %w", strContent, err)
	}
	return limit, true, nil
}

// findCgroupPath determines the absolute path to the cgroup v2 directory
// for the current process.
func findCgroupPath() (string, error) {
	// cgroupV2Root is the standard mount point for the cgroup v2 unified hierarchy.
	const cgroupV2Root = "/sys/fs/cgroup"

	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", fmt.Errorf("could not open /proc/self/cgroup: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// For cgroup v2, the line format is "0::/path/to/cgroup"
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" {
			// This is the unified hierarchy line.
			// The path is relative to the cgroup root.
			return filepath.Join(cgroupV2Root, parts[2]), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error scanning /proc/self/cgroup: %w", err)
	}

	return "", errors.New("cgroup v2 path not found in /proc/self/cgroup; this system may be using cgroup v1 or not have cgroups enabled")
}

// readCPUStat reads the usage_usec value from a cpu.stat file.
func readCPUStat(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[0] == "usage_usec" {
			return strconv.ParseUint(parts[1], 10, 64)
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return 0, fmt.Errorf("could not find 'usage_usec' in %s", path)
}

// readUintFromFile reads a single unsigned integer from a file.
func readUintFromFile(path string) (uint64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	strContent := strings.TrimSpace(string(content))
	val, err := strconv.ParseUint(strContent, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse uint from '%s': %w", strContent, err)
	}
	return val, nil
}
