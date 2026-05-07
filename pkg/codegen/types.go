package codegen

import (
	"github.com/tgoodwin/monolift/pkg/activation"
)

const (
	GeneratorVersion = "SPRINT-0041"
	ManifestName     = "monolift_lift_manifest.json"
)

// Plan is the transport-specific generation contract for one activation-path
// cut point.
type Plan struct {
	SourceModuleRoot string
	SourceModulePath string
	OutputDir        string
	ServiceName      string
	EnvServiceName   string

	CutPoint CutPoint
	Incoming IncomingCall

	BoundaryParams      []Param
	ReconstructedParams []ReconstructedParam
	Results             []Result
	ReturnCodec         ReturnCodec

	ServerPath   string
	ClientPath   string
	ManifestPath string

	Admission AdmissionVerdict
}

type CutPoint struct {
	PackagePath string
	PackageName string
	PackageDir  string
	FuncName    string
	Receiver    string
	File        string
	Line        int
	Column      int
	Key         activation.FunctionKey
}

type IncomingCall struct {
	File   string
	Line   int
	Column int
}

type ParamKind string

const (
	ParamBoundary      ParamKind = "boundary"
	ParamReconstructed ParamKind = "reconstructed"
)

type Param struct {
	Name             string
	JSONName         string
	GoType           string
	QualifiedGoType  string
	TypePackagePath  string
	TypePackageAlias string
	Codec            Codec
	Index            int
	Classification   activation.BoundaryDataClass
}

type ReconstructedParam struct {
	Param
	Reconstructor Reconstructor
}

type Result struct {
	Name             string
	JSONName         string
	GoType           string
	QualifiedGoType  string
	TypePackagePath  string
	TypePackageAlias string
	Codec            Codec
	Index            int
}

type Codec string

const (
	CodecPrimitive             Codec = "primitive"
	CodecJSON                  Codec = "json"
	CodecLocalizedErrorWrapper Codec = "localized_error_wrapper"
)

type ReturnCodec struct {
	Kind     Codec
	Nullable bool
	GoType   string
}

type Reconstructor struct {
	ID                      string
	Type                    string
	Imports                 []string
	ConstructorPackagePath  string
	ConstructorPackageAlias string
	ConstructorName         string
	CloseSource             string
}

type Artifact struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Content []byte `json:"-"`
}

type Manifest struct {
	GeneratorVersion string           `json:"generator_version"`
	ServiceName      string           `json:"service_name"`
	Cut              ManifestCut      `json:"cut"`
	ServerPath       string           `json:"server_path"`
	StubPath         string           `json:"stub_path,omitempty"`
	PatchedFile      string           `json:"patched_file,omitempty"`
	Artifacts        []ManifestEntry  `json:"artifacts"`
	Admission        AdmissionVerdict `json:"admission"`
}

type ManifestCut struct {
	PackagePath string `json:"package_path"`
	Function    string `json:"function"`
	Receiver    string `json:"receiver,omitempty"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
}

type ManifestEntry struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type AdmissionVerdict struct {
	Accepted bool                     `json:"accepted"`
	Reasons  []string                 `json:"reasons,omitempty"`
	Refusals []AdmissionRefusal       `json:"refusals,omitempty"`
	Cut      *activation.CutCandidate `json:"cut,omitempty"`
}

type AdmissionRefusal struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}
