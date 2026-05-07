package codegen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

// fixtureJSON is the on-disk format for cached pipeline fixtures under testdata/.
//
// Regenerate with:
//
//	MONOLIFT_UPDATE_FIXTURES=1 go test ./pkg/codegen/ -run TestGenerateFixtures -timeout 5m
type fixtureJSON struct {
	Report reportv2.Report      `json:"report"`
	Cut    activation.CutResult `json:"cut"`
}

type Fixture struct {
	Report reportv2.Report
	Cut    activation.CutResult
}

func SanitizeHTMLFixtureWithSource(repoRoot, moduleRoot string) Fixture {
	f := loadFixture("sanitizehtml")
	f.Report.BuildConfig.ModuleRoot = moduleRoot
	return f
}

func SanitizeHTMLFixture(repoRoot string) Fixture {
	f := loadFixture("sanitizehtml")
	f.Report.BuildConfig.ModuleRoot = filepath.Join(repoRoot, "evaluation", "miniflux")
	return f
}

func RefreshFeedFixtureWithSource(repoRoot, moduleRoot string) Fixture {
	f := loadFixture("refreshfeed")
	f.Report.BuildConfig.ModuleRoot = moduleRoot
	return f
}

func RefreshFeedFixture(repoRoot string) Fixture {
	f := loadFixture("refreshfeed")
	f.Report.BuildConfig.ModuleRoot = filepath.Join(repoRoot, "evaluation", "miniflux")
	return f
}

func loadFixture(name string) Fixture {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	path := filepath.Join(dir, "testdata", name+"_fixture.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("codegen: load fixture %s: %v", name, err))
	}
	var data fixtureJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		panic(fmt.Sprintf("codegen: parse fixture %s: %v", name, err))
	}
	return Fixture{
		Report: data.Report,
		Cut:    data.Cut,
	}
}
