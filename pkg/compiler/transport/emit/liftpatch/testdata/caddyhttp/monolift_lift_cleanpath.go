package caddyhttp

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

const monoliftLiftFailureSentinel = "\x00MONOLIFT_LIFT_FAILED\x00"

var (
	monoliftLiftEnabled  = os.Getenv("MONOLIFT_LIFT_CLEANPATH") == "on"
	monoliftLiftFailOpen = os.Getenv("MONOLIFT_LIFT_FAILMODE") == "open"
	monoliftLiftEndpoint = func() string {
		if v := os.Getenv("MONOLIFT_LIFT_CLEANPATH_ENDPOINT"); v != "" {
			return v
		}
		return "http://monolift-extracted-cleanpath:8081/invoke"
	}()
	monoliftLiftClient = &http.Client{
		Timeout:   2 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: 16},
	}
)

func monoliftLiftCleanPath(p string, collapseSlashes bool) (string, bool) {
	payload, err := json.Marshal(struct {
		P               string `json:"p"`
		CollapseSlashes bool   `json:"collapse_slashes"`
		InvocationID    string `json:"invocation_id,omitempty"`
	}{
		P:               p,
		CollapseSlashes: collapseSlashes,
	})
	if err != nil {
		log.Printf("monolift cleanpath remote error: marshal: %v", err)
		var zero string
		return zero, false
	}
	req, err := http.NewRequest("POST", monoliftLiftEndpoint, bytes.NewReader(payload))
	if err != nil {
		log.Printf("monolift cleanpath remote error: newrequest: %v", err)
		var zero string
		return zero, false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := monoliftLiftClient.Do(req)
	if err != nil {
		log.Printf("monolift cleanpath remote error: do: %v", err)
		var zero string
		return zero, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("monolift cleanpath remote error: status %d", resp.StatusCode)
		var zero string
		return zero, false
	}
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Printf("monolift cleanpath remote error: decode: %v", err)
		var zero string
		return zero, false
	}
	return out.Result, true
}
