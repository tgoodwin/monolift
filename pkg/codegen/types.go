package codegen

import (
	"github.com/tgoodwin/monolift/pkg/activation"
)

const (
	GeneratorVersion = "SPRINT-0050"
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

	ReceiverParam       *ReceiverSpec
	BoundaryParams      []Param
	ReconstructedParams []ReconstructedParam
	Results             []Result
	ReturnCodec         ReturnCodec

	ServerPath   string
	ClientPath   string
	ManifestPath string

	Deploy                  DeployOptions
	HostDockerfilePath      string
	ExtractedDockerfilePath string
	HostDeploymentPath      string
	HostServicePath         string
	ExtractedDeploymentPath string
	ExtractedServicePath    string
	SharedVolumeClaimPath   string

	Admission AdmissionVerdict

	// AdapterPlan is non-nil when the cut was admitted through boundary
	// adapter recovery. When set, codegen renders the adapter host wrapper
	// and normalized remote helper instead of the direct path.
	AdapterPlan *AdapterPlan `json:"adapter_plan,omitempty"`
}

type DeployOptions struct {
	HostImage             string
	ExtractedImage        string
	HostServiceName       string
	ExtractedServiceName  string
	HostPort              int
	ExtractedPort         int
	HostReadinessPath     string
	HostBuildPackage      string
	HostBinaryName        string
	HostBuildCommand      string
	HostRuntimeImage      string
	HostRuntimeSetup      []string
	HostArgs              []string
	HostEnvVars           []EnvVar
	ExtractedEnvVars      []EnvVar
	HostAssetCopies       []AssetCopy
	HostVolumeMounts      []VolumeMount
	ExtractedVolumeMounts []VolumeMount
	HostConfigMapVolumes  []ConfigMapVolume
	HostEmptyDirVolumes   []string
	SharedVolumeMounts    []SharedVolumeMount
	HostRunAsUser         int64
	ImagePullPolicy       string
}

type EnvVar struct {
	Name  string
	Value string
}

type AssetCopy struct {
	From string
	To   string
}

type VolumeMount struct {
	Name      string
	MountPath string
}

type ConfigMapVolume struct {
	Name          string
	ConfigMapName string
}

type SharedVolumeMount struct {
	Name           string
	ClaimName      string
	MountPath      string
	StorageRequest string
	// HostPath is node-local unless the cluster maps the same backing path into
	// every eligible node. Prefer PVCs for real clusters.
	HostPath string
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

type ReceiverPolicy string

const (
	ReceiverBoundary      ReceiverPolicy = "receiver_boundary"
	ReceiverZero          ReceiverPolicy = "receiver_zero"
	ReceiverFactory       ReceiverPolicy = "receiver_factory"
	ReceiverReconstructed ReceiverPolicy = "receiver_reconstructed"
)

type ReceiverSpec struct {
	GoType        string
	IsPointer     bool
	Policy        ReceiverPolicy
	FactoryFunc   string
	FactoryArgs   []string
	Codec         Codec
	Reconstructor Reconstructor
}

type Codec string

const (
	CodecPrimitive             Codec = "primitive"
	CodecJSON                  Codec = "json"
	CodecError                 Codec = "error"
	CodecLocalizedErrorWrapper Codec = "localized_error_wrapper"
	CodecStreamingBytes        Codec = "streaming_bytes"
)

type ReturnCodec struct {
	Kind     Codec
	Nullable bool
	GoType   string
}

type Reconstructor struct {
	ID                       string
	Type                     string
	Imports                  []string
	ConstructorPkg           string
	ConstructorFunc          string
	ConstructorArgOrder      []string
	ConstructorPackagePath   string
	ConstructorPackageAlias  string
	ConstructorName          string
	InitLines                []string
	StartupProbeLines        []string
	ConstructorLines         []string
	CloseSource              string
	ExtractedEnvVars         []EnvVar
	SharedVolumeMounts       []SharedVolumeMount
	RootRelativePathSuffixes []string
}

// AdapterTransport describes how adapted payloads are carried across the
// network boundary. inline_json_bytes is the only transport with a renderer
// in SPRINT-0051; staged_object is reserved for future use.
type AdapterTransport string

const (
	AdapterTransportInlineJSONBytes AdapterTransport = "inline_json_bytes"
	AdapterTransportStagedObject    AdapterTransport = "staged_object"
)

// AdapterPattern describes a single input or output transform in an
// AdapterPlan. The pattern name identifies the adapter library entry
// (e.g. "multipart_file_read_all", "bytes_reader_return") and the
// fields record what was matched.
type AdapterPattern struct {
	Name      string `json:"name"`
	ParamName string `json:"param_name,omitempty"`
	FromType  string `json:"from_type"`
	ToType    string `json:"to_type"`
}

// AdapterBodyRewrite describes the AST prologue replacement applied to
// the helper body when the adapter normalizes the remote signature.
type AdapterBodyRewrite struct {
	Description string `json:"description"`
	FromPattern string `json:"from_pattern,omitempty"`
	ToPattern   string `json:"to_pattern,omitempty"`
}

// AdapterProof records the discharge of one static feasibility obligation.
type AdapterProof struct {
	Obligation string `json:"obligation"`
	Satisfied  bool   `json:"satisfied"`
	Detail     string `json:"detail,omitempty"`
}

// AdapterPlan is the explicit IR for a boundary-normalized cut. When
// attached to a Plan, codegen renders a host wrapper preserving the
// original signature and a normalized remote helper with finite-value
// parameters and returns. The plan is JSON-tagged for manifest/debug
// emission and carries the proof obligations that justified admission.
type AdapterPlan struct {
	SourceFunction   string             `json:"source_function"`
	HostSignature    string             `json:"host_signature"`
	RemoteSignature  string             `json:"remote_signature"`
	InputTransforms  []AdapterPattern   `json:"input_transforms,omitempty"`
	BodyRewrite      AdapterBodyRewrite `json:"body_rewrite"`
	OutputTransforms []AdapterPattern   `json:"output_transforms,omitempty"`
	Proofs           []AdapterProof     `json:"proofs,omitempty"`
	TransportPolicy  AdapterTransport   `json:"transport_policy"`
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
	Deploy           ManifestDeploy   `json:"deploy"`
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

type ManifestDeploy struct {
	HostResourceName       string `json:"host_resource_name,omitempty"`
	ExtractedResourceName  string `json:"extracted_resource_name,omitempty"`
	HostImage              string `json:"host_image,omitempty"`
	ExtractedImage         string `json:"extracted_image,omitempty"`
	HostPort               int    `json:"host_port,omitempty"`
	ExtractedPort          int    `json:"extracted_port,omitempty"`
	EnvServiceName         string `json:"env_service_name,omitempty"`
	EnvVarPrefix           string `json:"env_var_prefix,omitempty"`
	EndpointEnv            string `json:"endpoint_env,omitempty"`
	EndpointURL            string `json:"endpoint_url,omitempty"`
	HostReadinessPath      string `json:"host_readiness_path,omitempty"`
	ExtractedReadinessPath string `json:"extracted_readiness_path,omitempty"`
	ImagePullPolicy        string `json:"image_pull_policy,omitempty"`
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
