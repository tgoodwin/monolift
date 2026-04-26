package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultSampleMS    = 250
	defaultRSSLimitMB  = 4096
	defaultWallLimitS  = 900
	statusAccepted     = "accepted"
	statusKilledRSS    = "killed_rss"
	statusKilledTime   = "killed_time"
	statusRegressed    = "regressed"
	statusWorking      = "working"
	workingThresholdPC = 15.0
	stabilityLimitPC   = 10.0
)

type artifact struct {
	Label        string      `json:"label"`
	Command      []string    `json:"command"`
	SampleMS     int         `json:"sample_ms"`
	RSSLimitMB   int         `json:"rss_limit_mb"`
	WallLimitSec int         `json:"wall_limit_sec"`
	Runs         []runRecord `json:"runs"`
	Summary      summary     `json:"summary"`
	Host         hostInfo    `json:"host"`
}

type runRecord struct {
	Seed              int          `json:"seed"`
	ExitCode          int          `json:"exit_code"`
	Killed            bool         `json:"killed"`
	KillReason        string       `json:"kill_reason"`
	ElapsedSec        float64      `json:"elapsed_sec"`
	PeakTreeRSSKB     int64        `json:"peak_tree_rss_kb"`
	PeakProcessRSSKB  int64        `json:"peak_process_rss_kb"`
	PeakProcessComm   string       `json:"peak_process_comm"`
	PeakProcessPID    int          `json:"peak_process_pid,omitempty"`
	PeakTreeProcesses []processRSS `json:"peak_tree_processes,omitempty"`
	LastObservedRSSKB int64        `json:"last_observed_tree_rss_kb,omitempty"`
}

type summary struct {
	Status             string  `json:"status"`
	BaselineArtifact   string  `json:"baseline_artifact"`
	CandidateArtifact  string  `json:"candidate_artifact"`
	WorstPeakTreeRSSKB int64   `json:"worst_peak_tree_rss_kb"`
	DeltaPct           float64 `json:"delta_pct"`
	SpreadPct          float64 `json:"spread_pct"`
	StabilityLimitPct  float64 `json:"stability_limit_pct,omitempty"`
	StabilityOK        bool    `json:"stability_ok"`
}

type hostInfo struct {
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	NCPU       int    `json:"ncpu"`
	GoVersion  string `json:"go_version"`
	GOMAXPROCS int    `json:"gomaxprocs"`
}

type aggregateConfig struct {
	Label               string
	Output              string
	Baseline            string
	TargetReductionPct  float64
	AbsolutePeakLimitMB int
	StabilityLimitPct   float64
	RecordOnly          bool
}

type treeSample struct {
	TreeRSSKB    int64
	ProcessRSSKB int64
	ProcessComm  string
	ProcessPID   int
	Processes    []processRSS
}

type processRSS struct {
	PID   int    `json:"pid"`
	RSSKB int64  `json:"rss_kb"`
	Comm  string `json:"comm"`
}

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: memcheck <run|aggregate> [flags] -- <command|run-artifacts>")
	}

	switch os.Args[1] {
	case "run":
		if err := runCommand(os.Args[2:]); err != nil {
			fatalf("memcheck run: %v", err)
		}
	case "aggregate":
		if err := aggregateRuns(os.Args[2:]); err != nil {
			fatalf("memcheck aggregate: %v", err)
		}
	default:
		fatalf("unknown subcommand %q", os.Args[1])
	}
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	label := fs.String("label", "", "artifact label")
	output := fs.String("output", "", "output artifact path")
	sampleMS := fs.Int("sample-ms", envInt("MEMCHECK_POLL_INTERVAL_MS", defaultSampleMS), "poll interval in milliseconds")
	rssLimitMB := fs.Int("rss-limit-mb", envInt("MEMCHECK_RSS_LIMIT_MB", defaultRSSLimitMB), "RSS kill limit in MB")
	wallLimitSec := fs.Int("wall-limit-sec", envInt("MEMCHECK_WALL_LIMIT_SEC", defaultWallLimitS), "wall-clock kill limit in seconds")
	seed := fs.Int("seed", 0, "shuffle seed for this run")
	if err := fs.Parse(args); err != nil {
		return err
	}

	command := fs.Args()
	if *label == "" {
		return errors.New("missing -label")
	}
	if *output == "" {
		return errors.New("missing -output")
	}
	if len(command) == 0 {
		return errors.New("missing measured command after --")
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	start := time.Now()
	record := runRecord{Seed: *seed}
	art := artifact{
		Label:        *label,
		Command:      append([]string(nil), command...),
		SampleMS:     *sampleMS,
		RSSLimitMB:   *rssLimitMB,
		WallLimitSec: *wallLimitSec,
		Runs:         []runRecord{record},
		Summary: summary{
			CandidateArtifact: *output,
			StabilityOK:       true,
			Status:            statusAccepted,
		},
		Host: currentHost(),
	}

	if err := flushArtifact(*output, art); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	ticker := time.NewTicker(time.Duration(*sampleMS) * time.Millisecond)
	defer ticker.Stop()

	var (
		killed     bool
		killReason string
		exitCode   int
	)

	updateFromSample := func(sample treeSample) {
		if sample.TreeRSSKB > art.Runs[0].PeakTreeRSSKB {
			art.Runs[0].PeakTreeRSSKB = sample.TreeRSSKB
			art.Runs[0].PeakTreeProcesses = sample.Processes
		}
		if sample.ProcessRSSKB > art.Runs[0].PeakProcessRSSKB {
			art.Runs[0].PeakProcessRSSKB = sample.ProcessRSSKB
			art.Runs[0].PeakProcessComm = sample.ProcessComm
			art.Runs[0].PeakProcessPID = sample.ProcessPID
		}
		art.Runs[0].LastObservedRSSKB = sample.TreeRSSKB
		art.Runs[0].ElapsedSec = round1(time.Since(start).Seconds())
		art.Runs[0].Killed = killed
		art.Runs[0].KillReason = killReason
		art.Runs[0].ExitCode = exitCode
		art.Summary.WorstPeakTreeRSSKB = art.Runs[0].PeakTreeRSSKB
		art.Summary.Status = singleRunStatus(art.Runs[0])
	}

	for {
		select {
		case waitErr := <-waitCh:
			sample, sampleErr := sampleProcessTree(pgid)
			if sampleErr == nil {
				updateFromSample(sample)
			}
			art.Runs[0].ElapsedSec = round1(time.Since(start).Seconds())
			exitCode = exitCodeFromWait(waitErr)
			art.Runs[0].ExitCode = exitCode
			if !killed && waitErr != nil {
				art.Summary.Status = statusRegressed
			} else {
				art.Summary.Status = singleRunStatus(art.Runs[0])
			}
			return flushArtifact(*output, art)
		case <-ticker.C:
			sample, sampleErr := sampleProcessTree(pgid)
			if sampleErr != nil {
				if processGone(sampleErr) {
					continue
				}
				return sampleErr
			}
			updateFromSample(sample)

			if !killed && *rssLimitMB > 0 && sample.TreeRSSKB > int64(*rssLimitMB)*1024 {
				killed = true
				killReason = "rss_limit"
				art.Runs[0].Killed = true
				art.Runs[0].KillReason = killReason
				if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
					return err
				}
			}

			if !killed && *wallLimitSec > 0 && time.Since(start) > time.Duration(*wallLimitSec)*time.Second {
				killed = true
				killReason = "wall_limit"
				art.Runs[0].Killed = true
				art.Runs[0].KillReason = killReason
				if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
					return err
				}
			}

			if err := flushArtifact(*output, art); err != nil {
				return err
			}
		}
	}
}

func aggregateRuns(args []string) error {
	fs := flag.NewFlagSet("aggregate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := aggregateConfig{}
	fs.StringVar(&cfg.Label, "label", "", "artifact label")
	fs.StringVar(&cfg.Output, "output", "", "output artifact path")
	fs.StringVar(&cfg.Baseline, "baseline", "", "baseline artifact path")
	fs.Float64Var(&cfg.TargetReductionPct, "target-reduction-pct", 0, "stage acceptance target improvement in percent")
	fs.IntVar(&cfg.AbsolutePeakLimitMB, "absolute-peak-limit-mb", 0, "optional absolute peak limit in MB")
	fs.Float64Var(&cfg.StabilityLimitPct, "stability-limit-pct", stabilityLimitPC, "maximum allowed peak RSS spread across runs")
	fs.BoolVar(&cfg.RecordOnly, "record-only", false, "record only without ratchet gating")
	if err := fs.Parse(args); err != nil {
		return err
	}

	runArtifacts := fs.Args()
	if cfg.Label == "" {
		return errors.New("missing -label")
	}
	if cfg.Output == "" {
		return errors.New("missing -output")
	}
	if len(runArtifacts) == 0 {
		return errors.New("missing run artifact paths after --")
	}

	combined := artifact{
		Label: cfg.Label,
		Host:  currentHost(),
		Summary: summary{
			CandidateArtifact: cfg.Output,
			StabilityOK:       true,
		},
	}

	for _, path := range runArtifacts {
		runArt, err := readArtifact(path)
		if err != nil {
			return err
		}
		if len(runArt.Runs) != 1 {
			return fmt.Errorf("run artifact %s has %d runs; want 1", path, len(runArt.Runs))
		}
		if len(combined.Command) == 0 {
			combined.Command = append([]string(nil), runArt.Command...)
			combined.SampleMS = runArt.SampleMS
			combined.RSSLimitMB = runArt.RSSLimitMB
			combined.WallLimitSec = runArt.WallLimitSec
			combined.Host = runArt.Host
		}
		combined.Runs = append(combined.Runs, runArt.Runs[0])
	}

	var baseline *artifact
	if cfg.Baseline != "" {
		loaded, err := readArtifact(cfg.Baseline)
		if err != nil {
			return err
		}
		baseline = &loaded
		combined.Summary.BaselineArtifact = cfg.Baseline
	}

	combined.Summary.WorstPeakTreeRSSKB = worstPeak(combined.Runs)
	combined.Summary.SpreadPct = spreadPct(combined.Runs)
	combined.Summary.StabilityLimitPct = cfg.StabilityLimitPct
	combined.Summary.StabilityOK = combined.Summary.SpreadPct <= cfg.StabilityLimitPct
	combined.Summary.DeltaPct = 0

	if baseline != nil && baseline.Summary.WorstPeakTreeRSSKB > 0 {
		combined.Summary.DeltaPct = round1((float64(combined.Summary.WorstPeakTreeRSSKB-baseline.Summary.WorstPeakTreeRSSKB) / float64(baseline.Summary.WorstPeakTreeRSSKB)) * 100)
	}

	combined.Summary.Status = determineAggregateStatus(combined.Runs, baseline, cfg, combined.Summary)
	return flushArtifact(cfg.Output, combined)
}

func determineAggregateStatus(runs []runRecord, baseline *artifact, cfg aggregateConfig, s summary) string {
	for _, run := range runs {
		if run.Killed {
			switch run.KillReason {
			case "rss_limit":
				return statusKilledRSS
			case "wall_limit":
				return statusKilledTime
			default:
				return statusRegressed
			}
		}
		if run.ExitCode != 0 {
			return statusRegressed
		}
	}

	if !s.StabilityOK {
		return statusRegressed
	}

	if cfg.RecordOnly || baseline == nil || baseline.Summary.WorstPeakTreeRSSKB == 0 {
		return statusAccepted
	}

	if cfg.AbsolutePeakLimitMB > 0 && s.WorstPeakTreeRSSKB > int64(cfg.AbsolutePeakLimitMB)*1024 {
		return statusRegressed
	}
	if cfg.AbsolutePeakLimitMB > 0 && cfg.TargetReductionPct <= 0 {
		return statusAccepted
	}

	improvementPct := -s.DeltaPct
	if cfg.TargetReductionPct <= 0 {
		if improvementPct >= 0 {
			return statusAccepted
		}
		return statusRegressed
	}
	if improvementPct >= cfg.TargetReductionPct {
		return statusAccepted
	}
	if improvementPct >= workingThresholdPC {
		return statusWorking
	}
	return statusRegressed
}

func sampleProcessTree(pgid int) (treeSample, error) {
	cmd := exec.Command("ps", "-o", "pid=,rss=,comm=", "-g", strconv.Itoa(pgid))
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(bytes.TrimSpace(exitErr.Stderr)) == 0 && len(bytes.TrimSpace(output)) == 0 {
			return treeSample{}, err
		}
		return treeSample{}, err
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var sample treeSample
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		rssKB, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		comm := strings.Join(fields[2:], " ")
		sample.TreeRSSKB += rssKB
		sample.Processes = append(sample.Processes, processRSS{PID: pid, RSSKB: rssKB, Comm: comm})
		if rssKB > sample.ProcessRSSKB {
			sample.ProcessRSSKB = rssKB
			sample.ProcessComm = comm
			sample.ProcessPID = pid
		}
	}
	sort.Slice(sample.Processes, func(i, j int) bool {
		return sample.Processes[i].RSSKB > sample.Processes[j].RSSKB
	})
	if len(sample.Processes) > 8 {
		sample.Processes = sample.Processes[:8]
	}
	return sample, nil
}

func processGone(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

func singleRunStatus(run runRecord) string {
	if run.Killed {
		switch run.KillReason {
		case "rss_limit":
			return statusKilledRSS
		case "wall_limit":
			return statusKilledTime
		default:
			return statusRegressed
		}
	}
	if run.ExitCode != 0 {
		return statusRegressed
	}
	return statusAccepted
}

func worstPeak(runs []runRecord) int64 {
	var worst int64
	for _, run := range runs {
		if run.PeakTreeRSSKB > worst {
			worst = run.PeakTreeRSSKB
		}
	}
	return worst
}

func spreadPct(runs []runRecord) float64 {
	if len(runs) == 0 {
		return 0
	}
	minPeak := runs[0].PeakTreeRSSKB
	maxPeak := runs[0].PeakTreeRSSKB
	for _, run := range runs[1:] {
		if run.PeakTreeRSSKB < minPeak {
			minPeak = run.PeakTreeRSSKB
		}
		if run.PeakTreeRSSKB > maxPeak {
			maxPeak = run.PeakTreeRSSKB
		}
	}
	if maxPeak == 0 {
		return 0
	}
	return round1((float64(maxPeak-minPeak) / float64(maxPeak)) * 100)
}

func currentHost() hostInfo {
	return hostInfo{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		NCPU:       runtime.NumCPU(),
		GoVersion:  runtime.Version(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
	}
}

func readArtifact(path string) (artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return artifact{}, err
	}
	var art artifact
	if err := json.Unmarshal(data, &art); err != nil {
		return artifact{}, err
	}
	return art, nil
}

func flushArtifact(path string, art artifact) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".memcheck-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func exitCodeFromWait(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
