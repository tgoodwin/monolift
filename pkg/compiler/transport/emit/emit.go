package emit

import (
	"errors"
	"fmt"

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
	ExpectedSignature string
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
