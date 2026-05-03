package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest describes the pinned evaluation targets.
type Manifest struct {
	PinnedAt string            `yaml:"pinned_at"`
	Targets  []ManifestItem    `yaml:"targets"`
	ByName   map[string]Target `yaml:"-"`
}

// ManifestItem is one target entry as it appears in evaluation/MANIFEST.yaml.
type ManifestItem struct {
	Name     string `yaml:"name"`
	Upstream string `yaml:"upstream"`
	SHA      string `yaml:"sha"`
	GoFiles  int    `yaml:"go_files"`
}

// Target is a manifest target enriched with local evaluation paths and the
// command package pattern used as the RTA root.
type Target struct {
	Name           string `json:"name"`
	Upstream       string `json:"upstream"`
	SHA            string `json:"sha"`
	GoFiles        int    `json:"go_files"`
	ProjectDir     string `json:"project_dir"`
	WorkDir        string `json:"work_dir"`
	ModulePath     string `json:"module_path"`
	PackagePattern string `json:"package_pattern"`
}

// Case pairs one ground-truth trace with its pinned evaluation target.
type Case struct {
	Trace  Trace  `json:"trace"`
	Target Target `json:"target"`
}

// LoadManifest parses evaluation/MANIFEST.yaml and enriches it with local
// project directories and package patterns.
func LoadManifest(path, evaluationRoot string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	manifest.ByName = make(map[string]Target, len(manifest.Targets))
	for _, item := range manifest.Targets {
		target := Target{
			Name:           item.Name,
			Upstream:       item.Upstream,
			SHA:            item.SHA,
			GoFiles:        item.GoFiles,
			ProjectDir:     filepath.Join(evaluationRoot, item.Name),
			PackagePattern: defaultPackagePattern(item.Name),
		}
		target.WorkDir = target.ProjectDir
		if item.Name == "mattermost" {
			target.WorkDir = filepath.Join(target.ProjectDir, "server")
		}
		target.ModulePath = readModulePath(target.WorkDir)
		if target.PackagePattern == "" {
			return Manifest{}, fmt.Errorf("no package pattern for evaluation target %q", item.Name)
		}
		manifest.ByName[item.Name] = target
	}
	return manifest, nil
}

func readModulePath(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1]
		}
	}
	return ""
}

// EnumerateCases returns every structured trace paired with its evaluation
// target, sorted by project and candidate number.
func EnumerateCases(tracesDir, manifestPath, evaluationRoot string) ([]Case, error) {
	traces, err := LoadTraces(tracesDir)
	if err != nil {
		return nil, err
	}
	manifest, err := LoadManifest(manifestPath, evaluationRoot)
	if err != nil {
		return nil, err
	}
	cases := make([]Case, 0, len(traces))
	for _, trace := range traces {
		target, ok := manifest.ByName[trace.Project]
		if !ok {
			return nil, fmt.Errorf("trace %s references project %q not found in manifest", trace.ID, trace.Project)
		}
		cases = append(cases, Case{Trace: trace, Target: target})
	}
	sort.Slice(cases, func(i, j int) bool {
		if cases[i].Trace.Project != cases[j].Trace.Project {
			return cases[i].Trace.Project < cases[j].Trace.Project
		}
		if cases[i].Trace.CandidateNumber != cases[j].Trace.CandidateNumber {
			return cases[i].Trace.CandidateNumber < cases[j].Trace.CandidateNumber
		}
		return cases[i].Trace.ID < cases[j].Trace.ID
	})
	return cases, nil
}

func defaultPackagePattern(project string) string {
	switch project {
	case "caddy":
		return "./cmd/caddy"
	case "gitea":
		return "."
	case "listmonk":
		return "./cmd"
	case "mattermost":
		return "./cmd/mattermost"
	case "miniflux":
		return "."
	case "pocketbase":
		return "./examples/base"
	default:
		return ""
	}
}
