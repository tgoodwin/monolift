package codegen

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestStreamingBytesRoundTrip(t *testing.T) {
	original := []byte("Hello, streaming world! 🌍 binary: \x00\x01\x02\xff")

	// Client side: serialize io.Reader content as []byte in JSON request
	type request struct {
		Data []byte `json:"data"`
	}
	req := request{Data: original}
	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Server side: deserialize and reconstruct as io.ReadSeeker
	var decoded request
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	reader := bytes.NewReader(decoded.Data)

	// Verify round-trip: read from reconstructed reader matches original
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, original)
	}

	// Verify io.ReadSeeker supports Seek
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	got2, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll after Seek: %v", err)
	}
	if !bytes.Equal(got2, original) {
		t.Fatalf("round-trip after Seek mismatch: got %q, want %q", got2, original)
	}
}

func TestStreamingBytesAdmission(t *testing.T) {
	plan := streamingBytesServerPlan()
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if !verdict.Accepted {
		t.Fatalf("streaming bytes plan should be admitted, got refusals: %v", verdict.Refusals)
	}
}
