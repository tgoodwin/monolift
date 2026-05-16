package harness

import "testing"

func TestNormalizeBodyFields(t *testing.T) {
	transcript := Transcript{Steps: []Step{{
		BodyJSON: map[string]any{
			"id": "123",
			"nested": []any{
				map[string]any{"created_at": "2026-05-16T00:00:00Z", "value": "kept"},
			},
		},
	}}}

	got := transcript.Normalize(NormalizeIDs(), NormalizeTimestamps())
	body := got.Steps[0].BodyJSON.(map[string]any)
	if body["id"] != "<normalized>" {
		t.Fatalf("id = %v, want normalized", body["id"])
	}
	nested := body["nested"].([]any)[0].(map[string]any)
	if nested["created_at"] != "<normalized>" || nested["value"] != "kept" {
		t.Fatalf("nested = %#v", nested)
	}
}

func TestNormalizeGeneratedPaths(t *testing.T) {
	transcript := Transcript{Steps: []Step{{
		BodyJSON: map[string]any{
			"path": "created /tmp/monolift-e2e/target/run/output.txt",
		},
	}}}

	got := transcript.Normalize(NormalizeGeneratedPaths())
	body := got.Steps[0].BodyJSON.(map[string]any)
	if body["path"] != "created <generated-path>" {
		t.Fatalf("path = %q", body["path"])
	}
}
