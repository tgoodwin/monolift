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

	ctx, ok := cleanPathEmitContext(result.Report)
	if !ok {
		return &result.Report, nil, transportResult, nil
	}
	extracted, err := emit.Emit(transport.Selection{Template: transport.TemplateHTTPJSON}, ctx)
	if err != nil {
		return nil, nil, transport.Result{}, err
	}
	patch, err := liftpatch.Render(ctx)
	if err != nil {
		return nil, nil, transport.Result{}, err
	}
	return &result.Report, []emit.Artifact{extracted, patch}, transportResult, nil
}

func cleanPathEmitContext(report reportv2.Report) (emit.Context, bool) {
	for _, symbol := range report.Closure.IncludedSymbols {
		identity := symbol.Identity
		if identity.PackagePath == "github.com/caddyserver/caddy/v2/modules/caddyhttp" &&
			identity.ObjectName == "CleanPath" &&
			identity.Kind == "function" {
			return emit.Context{
				SymbolImportPath:   identity.PackagePath,
				ObjectName:         identity.ObjectName,
				ParamFields:        []emit.FieldSpec{{Name: "P", JSONName: "p", GoType: "string"}, {Name: "CollapseSlashes", JSONName: "collapse_slashes", GoType: "bool"}},
				ResultFields:       []emit.FieldSpec{{Name: "Result", JSONName: "result", GoType: "string"}},
				UpstreamModulePath: "github.com/caddyserver/caddy/v2",
				UpstreamLocalPath:  "../upstream",
				ServiceName:        "monolift-extracted-cleanpath",
				EnvVarPrefix:       "MONOLIFT_LIFT_CLEANPATH",
			}, true
		}
	}
	return emit.Context{}, false
}

func requireCleanPathEmitContext(report reportv2.Report) (emit.Context, error) {
	ctx, ok := cleanPathEmitContext(report)
	if !ok {
		return emit.Context{}, fmt.Errorf("caddy CleanPath not found in closure")
	}
	return ctx, nil
}
