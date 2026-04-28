package compiler

import (
	extractv2 "github.com/tgoodwin/monolift/pkg/compiler/extract"
	_ "github.com/tgoodwin/monolift/pkg/compiler/passes"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func Extract(sources []string, pragmas []*Pragma) (reportv2.Report, []Diagnostic, error) {
	req := extractRequest(sources, pragmas)

	result, err := extractv2.Analyze(req)
	if err != nil {
		return reportv2.Report{}, nil, err
	}

	diagnostics := make([]Diagnostic, 0, len(result.Diagnostics))
	for _, diagnostic := range result.Diagnostics {
		diagnostics = append(diagnostics, Diagnostic{
			Code:       diagnostic.Code,
			Severity:   Severity(diagnostic.Severity),
			Message:    diagnostic.Message,
			Span:       Span(diagnostic.Span),
			RuleIDs:    append([]string(nil), diagnostic.RuleIDs...),
			Suggestion: diagnostic.Suggestion,
		})
	}
	return result.Report, diagnostics, nil
}

func extractRequest(sources []string, pragmas []*Pragma) extractv2.Request {
	regions, _ := RegroupPragmas(pragmas)
	req := extractv2.Request{
		Sources: append([]string(nil), sources...),
		Pragmas: make([]extractv2.Pragma, 0, len(pragmas)),
		Regions: make([]extractv2.Region, 0, len(regions)),
	}
	for _, pragma := range pragmas {
		if pragma == nil {
			continue
		}
		req.Pragmas = append(req.Pragmas, extractPragma(pragma))
	}
	for _, region := range regions {
		req.Regions = append(req.Regions, extractRegion(region))
	}
	return req
}

func extractPragma(pragma *Pragma) extractv2.Pragma {
	return extractv2.Pragma{
		Name:         pragma.Name,
		Surface:      extractv2.Surface(pragma.Surface),
		Options:      cloneOptions(pragma.Options),
		Span:         extractv2.Span(pragma.Span),
		DeclName:     pragma.DeclName,
		DeclKind:     pragma.DeclKind,
		DeclIdentity: pragma.DeclIdentity,
	}
}

func extractRegion(region *Region) extractv2.Region {
	out := extractv2.Region{
		Name:      region.Name,
		Span:      extractv2.Span(region.Span),
		Mode:      region.Mode,
		Transport: region.Transport,
		Policy:    region.Policy,
		Dispatch:  region.Dispatch,
		Affinity:  region.Affinity,
		Roots:     make([]extractv2.RegionRoot, 0, len(region.Roots)),
	}
	for _, root := range region.Roots {
		if root == nil || root.Pragma == nil {
			continue
		}
		out.Roots = append(out.Roots, extractv2.RegionRoot{
			ID:     root.ID,
			Pragma: extractPragma(root.Pragma),
		})
	}
	return out
}

func cloneOptions(options map[string]string) map[string]string {
	if len(options) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(options))
	for key, value := range options {
		cloned[key] = value
	}
	return cloned
}
