package liftpatch

type PatchRequest struct {
	ModuleRoot        string
	PackageImportPath string
	PackageDir        string
	FuncName          string
	ExpectedSignature string
	PreludeSpec       PreludeSpec
	GeneratedFiles    []GeneratedFile
	SentinelIdent     string
}

type RegionPatchRequest struct {
	RegionName           string
	Symbols              []PatchSymbolRequest
	SharedGeneratedFiles []GeneratedFile
}

type PatchSymbolRequest struct {
	PackageImportPath string
	PackageDir        string
	File              string
	FuncName          string
	ReceiverType      string
	ExpectedSignature string
	Prelude           PreludeSpec
	SentinelIdent     string
	GeneratedFiles    []GeneratedFile
}

type PreludeSpec struct {
	GoSource        string
	RequiredImports []string
}

type GeneratedFile struct {
	Path    string
	Content []byte
}

type PatchResult struct {
	PatchedFile    string
	AddedImports   []string
	GeneratedFiles []string
	OriginalSHA256 string
	PatchedSHA256  string
	AlreadyApplied bool
}

type RegionPatchResult struct {
	Files          []PatchedFileResult
	GeneratedFiles []GeneratedFileResult
	Refused        *RegionPatchRefusal
}

type PatchedFileResult struct {
	Path           string
	OriginalSHA256 string
	PatchedSHA256  string
	AddedImports   []string
	AlreadyApplied bool
}

type GeneratedFileResult struct {
	Path   string
	SHA256 string
}

type RegionPatchRefusal struct {
	Kind    DiagnosticKind
	Message string
	Symbol  PatchSymbolRequest
}

type LiftPatchManifest struct {
	PackageImportPath string   `json:"package_import_path"`
	FilePath          string   `json:"file_path"`
	FunctionName      string   `json:"function_name"`
	ExpectedSignature string   `json:"expected_signature"`
	SentinelIdent     string   `json:"sentinel_identifier"`
	OriginalSHA256    string   `json:"original_sha256"`
	PatchedSHA256     string   `json:"patched_sha256"`
	GeneratedFiles    []string `json:"generated_files"`
}

type DiagnosticKind string

const (
	DiagnosticTargetNotFound       DiagnosticKind = "target_not_found"
	DiagnosticAmbiguousTarget      DiagnosticKind = "ambiguous_target"
	DiagnosticSignatureMismatch    DiagnosticKind = "signature_mismatch"
	DiagnosticGenericFunction      DiagnosticKind = "generic_function"
	DiagnosticMethodReceiver       DiagnosticKind = "method_receiver"
	DiagnosticNamedNakedReturn     DiagnosticKind = "named_naked_return"
	DiagnosticUnsupportedBuildTags DiagnosticKind = "unsupported_build_tags"
	DiagnosticIdentifierCollision  DiagnosticKind = "identifier_collision"
)

type DiagnosticError struct {
	Kind    DiagnosticKind
	Message string
}

func (e *DiagnosticError) Error() string {
	if e.Message == "" {
		return string(e.Kind)
	}
	return string(e.Kind) + ": " + e.Message
}
