package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

type WorkloadExecutor interface {
	Setup(ctx context.Context, host string) error
	Action(ctx context.Context, host string) (Transcript, error)
	Verify(ctx context.Context, host string, expected Transcript) error
}

type Workload struct {
	Invariants []Invariant
}

type Transcript struct {
	Steps []Step `json:"steps"`
}

type Step struct {
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Status   int               `json:"status"`
	Headers  map[string]string `json:"headers,omitempty"`
	BodyJSON any               `json:"bodyJSON,omitempty"`
}

type Invariant struct {
	Path    string
	Status  bool
	Headers []string
	Body    bool
}

type PairedTranscripts struct {
	Baseline Transcript
	Lifted   Transcript
}

type TranscriptNormalizer func(*Transcript)

func (w Workload) RunBoth(ctx context.Context, baselineURL, liftedURL string, exec WorkloadExecutor) (PairedTranscripts, error) {
	if err := exec.Setup(ctx, baselineURL); err != nil {
		return PairedTranscripts{}, StageError(2, "unknown", KindWorkload, "baseline setup failed: %v", err)
	}
	baseline, err := exec.Action(ctx, baselineURL)
	if err != nil {
		return PairedTranscripts{}, StageError(2, "unknown", KindWorkload, "baseline action failed: %v", err)
	}
	if err := exec.Setup(ctx, liftedURL); err != nil {
		return PairedTranscripts{}, StageError(8, "unknown", KindWorkload, "lifted setup failed: %v", err)
	}
	lifted, err := exec.Action(ctx, liftedURL)
	if err != nil {
		return PairedTranscripts{}, StageError(8, "unknown", KindWorkload, "lifted action failed: %v", err)
	}
	return PairedTranscripts{Baseline: baseline, Lifted: lifted}, nil
}

func (t Transcript) Normalize(normalizers ...TranscriptNormalizer) Transcript {
	clone := t
	clone.Steps = append([]Step(nil), t.Steps...)
	for _, normalize := range normalizers {
		normalize(&clone)
	}
	return clone
}

func (t Transcript) Compare(baseline, lifted Transcript, invariants []Invariant) error {
	if len(baseline.Steps) != len(lifted.Steps) {
		return fmt.Errorf("step count mismatch: baseline=%d lifted=%d", len(baseline.Steps), len(lifted.Steps))
	}
	for i := range baseline.Steps {
		bs, ls := baseline.Steps[i], lifted.Steps[i]
		invariant := invariantFor(bs.Path, invariants)
		if bs.Method != ls.Method || bs.Path != ls.Path {
			return fmt.Errorf("step[%d] request mismatch: %s %s vs %s %s", i, bs.Method, bs.Path, ls.Method, ls.Path)
		}
		if invariant.Status && bs.Status != ls.Status {
			return fmt.Errorf("step[%d] status mismatch: baseline=%d lifted=%d", i, bs.Status, ls.Status)
		}
		for _, header := range invariant.Headers {
			if bs.Headers[header] != ls.Headers[header] {
				return fmt.Errorf("step[%d] header %s mismatch: baseline=%q lifted=%q", i, header, bs.Headers[header], ls.Headers[header])
			}
		}
		if invariant.Body && !reflect.DeepEqual(normalizeJSON(bs.BodyJSON), normalizeJSON(ls.BodyJSON)) {
			return fmt.Errorf("step[%d] body mismatch: baseline=%v lifted=%v", i, bs.BodyJSON, ls.BodyJSON)
		}
	}
	return nil
}

func invariantFor(path string, invariants []Invariant) Invariant {
	for _, invariant := range invariants {
		if invariant.Path == path {
			return invariant
		}
	}
	return Invariant{Path: path, Status: true, Body: true}
}

func normalizeJSON(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return v
	}
	return out
}
