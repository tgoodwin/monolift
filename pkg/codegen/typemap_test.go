package codegen

import (
	"testing"
)

func TestTypeMapMinifluxSignatures(t *testing.T) {
	sanitizePlan, err := BuildPlan(SanitizeHTMLFixture(repoRoot(t)).Report, SanitizeHTMLFixture(repoRoot(t)).Cut)
	if err != nil {
		t.Fatal(err)
	}
	if sanitizePlan.BoundaryParams[0].Codec != CodecPrimitive {
		t.Fatalf("baseURL codec = %s", sanitizePlan.BoundaryParams[0].Codec)
	}
	if sanitizePlan.BoundaryParams[2].Codec != CodecJSON {
		t.Fatalf("sanitizerOptions codec = %s", sanitizePlan.BoundaryParams[2].Codec)
	}

	refreshPlan, err := BuildPlan(RefreshFeedFixture(repoRoot(t)).Report, RefreshFeedFixture(repoRoot(t)).Cut)
	if err != nil {
		t.Fatal(err)
	}
	for _, param := range refreshPlan.BoundaryParams {
		if param.Codec != CodecPrimitive {
			t.Fatalf("%s codec = %s, want primitive", param.Name, param.Codec)
		}
	}
	if refreshPlan.ReconstructedParams[0].Codec != CodecJSON {
		t.Fatalf("store codec = %s", refreshPlan.ReconstructedParams[0].Codec)
	}
	if refreshPlan.Results[0].Codec != CodecLocalizedErrorWrapper {
		t.Fatalf("result codec = %s", refreshPlan.Results[0].Codec)
	}
}

func TestJSONFieldNames(t *testing.T) {
	cases := map[string]string{
		"baseURL":      "base_url",
		"rawHTML":      "raw_html",
		"userID":       "user_id",
		"feedID":       "feed_id",
		"forceRefresh": "force_refresh",
		"store":        "store",
	}
	for input, want := range cases {
		if got := jsonFieldName("", input); got != want {
			t.Fatalf("jsonFieldName(%q) = %q, want %q", input, got, want)
		}
	}
	if got := jsonFieldName("SanitizeHTML", "rawHTML"); got != "input" {
		t.Fatalf("SanitizeHTML rawHTML json = %q", got)
	}
}

func TestByteSlicesRemainBoundaryParams(t *testing.T) {
	param := Param{
		Name:            "salt",
		GoType:          "[]byte",
		QualifiedGoType: "[]byte",
		Codec:           CodecJSON,
	}
	if !alwaysBoundaryParam(param) {
		t.Fatal("[]byte JSON param should remain a boundary param")
	}
}
