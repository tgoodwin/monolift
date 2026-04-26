package compiler

import (
	"fmt"

	extractv2 "github.com/tgoodwin/monolift/pkg/compiler/extract"
	_ "github.com/tgoodwin/monolift/pkg/compiler/passes"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"github.com/tgoodwin/monolift/pkg/compiler/transport"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit"
	_ "github.com/tgoodwin/monolift/pkg/compiler/transport/emit/httpjson"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit/liftpatch"
)

func ExtractWithTransport(sources []string, pragmas []*Pragma) (*reportv2.Report, []emit.Artifact, transport.Result, error) {
	req := extractRequest(sources, pragmas)
	result, err := extractv2.Analyze(req)
	if err != nil {
		return nil, nil, transport.Result{}, err
	}
	loaded, err := extractv2.LoadModule(req)
	if err != nil {
		return nil, nil, transport.Result{}, err
	}
	program, err := extractv2.BuildProgram(loaded)
	if err != nil {
		return nil, nil, transport.Result{}, err
	}
	transportResult, err := transport.Classify(loaded, program, result.Report.Root)
	if err != nil {
		return nil, nil, transport.Result{}, err
	}

	contexts := emitContexts(result.Report)
	if len(contexts) == 0 {
		return &result.Report, nil, transportResult, nil
	}
	artifacts := make([]emit.Artifact, 0, len(contexts)*2)
	for _, ctx := range contexts {
		extracted, err := emit.Emit(transport.Selection{Template: transport.TemplateHTTPJSON}, ctx)
		if err != nil {
			return nil, nil, transport.Result{}, err
		}
		patch, err := liftpatch.Render(ctx)
		if err != nil {
			return nil, nil, transport.Result{}, err
		}
		artifacts = append(artifacts, extracted, patch)
	}
	return &result.Report, artifacts, transportResult, nil
}

func cleanPathEmitContext(report reportv2.Report) (emit.Context, bool) {
	for _, ctx := range emitContexts(report) {
		if ctx.ObjectName == "CleanPath" {
			return ctx, true
		}
	}
	return emit.Context{}, false
}

func emitContexts(report reportv2.Report) []emit.Context {
	found := map[string]bool{}
	for _, symbol := range report.Closure.IncludedSymbols {
		identity := symbol.Identity
		if identity.Kind == "function" {
			found[identity.PackagePath+"."+identity.ObjectName] = true
		}
	}
	var contexts []emit.Context
	if found["github.com/caddyserver/caddy/v2/modules/caddyhttp.CleanPath"] {
		contexts = append(contexts, emit.Context{
			SymbolImportPath:   "github.com/caddyserver/caddy/v2/modules/caddyhttp",
			ObjectName:         "CleanPath",
			ParamFields:        []emit.FieldSpec{{Name: "P", JSONName: "p", GoType: "string"}, {Name: "CollapseSlashes", JSONName: "collapse_slashes", GoType: "bool"}},
			ResultFields:       []emit.FieldSpec{{Name: "Result", JSONName: "result", GoType: "string"}},
			UpstreamModulePath: "github.com/caddyserver/caddy/v2",
			ServiceName:        "monolift-extracted-cleanpath",
			EnvVarPrefix:       "MONOLIFT_LIFT_CLEANPATH",
		})
	}
	if found["github.com/caddyserver/caddy/v2/internal/metrics.SanitizeMethod"] {
		contexts = append(contexts, emit.Context{
			SymbolImportPath:   "github.com/caddyserver/caddy/v2/internal/metrics",
			ObjectName:         "SanitizeMethod",
			ParamFields:        []emit.FieldSpec{{Name: "M", JSONName: "m", GoType: "string"}},
			ResultFields:       []emit.FieldSpec{{Name: "Result", JSONName: "result", GoType: "string"}},
			UpstreamModulePath: "github.com/caddyserver/caddy/v2",
			ServiceName:        "monolift-extracted-sanitizemethod",
			EnvVarPrefix:       "MONOLIFT_LIFT_SANITIZEMETHOD",
		})
	}
	if found["miniflux.app/v2/internal/reader/readingtime.EstimateReadingTime"] {
		contexts = append(contexts, emit.Context{
			SymbolImportPath:   "miniflux.app/v2/internal/reader/readingtime",
			ObjectName:         "EstimateReadingTime",
			ParamFields:        []emit.FieldSpec{{Name: "Content", JSONName: "content", GoType: "string"}, {Name: "DefaultReadingSpeed", JSONName: "default_reading_speed", GoType: "int"}, {Name: "CjkReadingSpeed", JSONName: "cjk_reading_speed", GoType: "int"}},
			ResultFields:       []emit.FieldSpec{{Name: "ReadingTime", JSONName: "reading_time", GoType: "int"}},
			UpstreamModulePath: "miniflux.app/v2",
			ServiceName:        "monolift-extracted-estimatereadingtime",
			EnvVarPrefix:       "MONOLIFT_LIFT_ESTIMATEREADINGTIME",
		})
	}
	return contexts
}

func requireCleanPathEmitContext(report reportv2.Report) (emit.Context, error) {
	ctx, ok := cleanPathEmitContext(report)
	if !ok {
		return emit.Context{}, fmt.Errorf("caddy CleanPath not found in closure")
	}
	return ctx, nil
}
