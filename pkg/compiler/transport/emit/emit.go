package emit

import (
	"errors"
	"fmt"

	"github.com/tgoodwin/monolift/pkg/compiler/extract/bootpath"
	"github.com/tgoodwin/monolift/pkg/compiler/surface"
	"github.com/tgoodwin/monolift/pkg/compiler/transport"
)

var ErrTemplateUnsupported = errors.New("transport emit template unsupported")

type FieldSpec struct {
	Name     string
	JSONName string
	GoType   string
}

type Context struct {
	SymbolImportPath   string
	ObjectName         string
	ParamFields        []FieldSpec
	ResultFields       []FieldSpec
	UpstreamModulePath string
	UpstreamLocalPath  string
	ServiceName        string
	EnvVarPrefix       string
}

type PatchRoute string

const (
	PatchRouteSymbol PatchRoute = "symbol"
	PatchRouteRegion PatchRoute = "region"
)

type RegionPlan struct {
	Region             RegionSpec
	Surface            surface.RegionSurface
	Boot               bootpath.BootSpec
	ServiceName        string
	ExtractedAddress   string
	PackageImportPath  string
	PackageDir         string
	SharedPackageFiles []string
}

type RegionSpec struct {
	Name  string
	Roots []RegionRootSpec
}

type RegionRootSpec struct {
	FuncName          string
	ReceiverType      string
	File              string
	ExpectedSignature string
	Route             string
}

func PatchRouteForRegion(plan RegionPlan) PatchRoute {
	if len(plan.Region.Roots) > 1 {
		return PatchRouteRegion
	}
	for _, root := range plan.Region.Roots {
		if root.ReceiverType != "" {
			return PatchRouteRegion
		}
	}
	return PatchRouteSymbol
}

type Artifact struct {
	Files        map[string][]byte
	Manifest     Manifest
	HostPatchOps []HostPatchOp
}

type Manifest struct {
	ServiceName string   `json:"service_name"`
	Files       []string `json:"files"`
}

type HostPatchOp struct {
	ModuleRoot        string
	PackageImportPath string
	PackageDir        string
	FuncName          string
	ReceiverType      string
	ExpectedSignature string
	PreludeSource     string
	GeneratedFiles    []string
	SentinelIdent     string
}

type Renderer func(Context) (Artifact, error)

var renderers = map[transport.Template]Renderer{}

func Register(template transport.Template, renderer Renderer) {
	renderers[template] = renderer
}

func Emit(sel transport.Selection, ctx Context) (Artifact, error) {
	renderer, ok := renderers[sel.Template]
	if !ok {
		return Artifact{}, fmt.Errorf("%w: %s", ErrTemplateUnsupported, sel.Template)
	}
	return renderer(ctx)
}

func TemplateForSurface(regionSurface surface.RegionSurface) transport.Template {
	if regionSurface.Category == surface.SurfaceSession || regionSurface.WireProtocol == surface.WireProtocolStreamProxy {
		return transport.TemplateStreamProxy
	}
	return transport.TemplateHTTPJSON
}
