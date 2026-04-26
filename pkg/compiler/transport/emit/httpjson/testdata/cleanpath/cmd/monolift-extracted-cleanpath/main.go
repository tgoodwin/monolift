package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type invokeRequest struct {
	P               string `json:"p"`
	CollapseSlashes bool   `json:"collapse_slashes"`
	InvocationID    string `json:"invocation_id,omitempty"`
}

type invokeResponse struct {
	Result string `json:"result"`
}

type invocationRecord struct {
	ID              int64     `json:"id"`
	InvocationID    string    `json:"invocation_id,omitempty"`
	P               string    `json:"p"`
	CollapseSlashes bool      `json:"collapse_slashes"`
	Result          string    `json:"result"`
	Timestamp       time.Time `json:"timestamp"`
}

type invocationStore struct {
	mu      sync.Mutex
	nextID  int64
	records []invocationRecord
}

const maxInvocationRecords = 256

var (
	counter int64
	records invocationStore
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "run healthcheck")
	flag.Parse()
	if *healthcheck {
		if err := runHealthcheck(); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	http.HandleFunc("/invoke", handleInvoke)
	http.HandleFunc("/calls", handleCalls)
	http.HandleFunc("/invocations", handleInvocations)
	http.HandleFunc("/healthz", handleHealthz)
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func runHealthcheck() error {
	resp, err := http.Get("http://127.0.0.1:8081/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck status %d", resp.StatusCode)
	}
	return nil
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in invokeRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.InvocationID == "" {
		in.InvocationID = r.Header.Get("X-Monolift-Invocation-ID")
	}
	atomic.AddInt64(&counter, 1)
	result := caddyhttp.CleanPath(in.P, in.CollapseSlashes)
	record := records.append(in, result)
	log.Printf("LIFT_INVOKE service=monolift-extracted-cleanpath id=%s result=%v", in.InvocationID, result)
	writeJSON(w, http.StatusOK, invokeResponse{Result: record.Result})
}

func handleCalls(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Count int64 `json:"count"`
	}{Count: atomic.LoadInt64(&counter)})
}

func handleInvocations(w http.ResponseWriter, r *http.Request) {
	since := int64(0)
	if value := r.URL.Query().Get("since"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since")
			return
		}
		since = parsed
	}
	writeJSON(w, http.StatusOK, struct {
		Records []invocationRecord `json:"records"`
	}{Records: records.since(since)})
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *invocationStore) append(in invokeRequest, result string) invocationRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	record := invocationRecord{
		ID:              s.nextID,
		InvocationID:    in.InvocationID,
		P:               in.P,
		CollapseSlashes: in.CollapseSlashes,
		Result:          result,
		Timestamp:       time.Now().UTC(),
	}
	s.records = append(s.records, record)
	if len(s.records) > maxInvocationRecords {
		copy(s.records, s.records[len(s.records)-maxInvocationRecords:])
		s.records = s.records[:maxInvocationRecords]
	}
	return record
}

func (s *invocationStore) since(id int64) []invocationRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]invocationRecord, 0, len(s.records))
	for _, record := range s.records {
		if record.ID > id {
			out = append(out, record)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}
