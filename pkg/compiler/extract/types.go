package extract

import (
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"github.com/tgoodwin/monolift/pkg/compiler/surface"
)

type Surface string

const (
	SurfaceInterface Surface = "interface"
	SurfaceFunction  Surface = "function"
	SurfaceMethod    Surface = "method"
	SurfaceStruct    Surface = "struct"
)

type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Span struct {
	Filename  string
	Line      int
	Column    int
	EndLine   int
	EndColumn int
}

type Pragma struct {
	Name         string
	Surface      Surface
	Options      map[string]string
	Span         Span
	DeclName     string
	DeclKind     string
	DeclIdentity string
}

type RegionRoot struct {
	ID     string
	Pragma Pragma
}

type Region struct {
	Name      string
	Roots     []RegionRoot
	Span      Span
	Mode      string
	Transport string
	Policy    string
	Dispatch  string
	Affinity  string
}

type Diagnostic struct {
	Code       string
	Severity   Severity
	Message    string
	Span       Span
	RuleIDs    []string
	Suggestion string
}

type Request struct {
	Sources []string
	Pragmas []Pragma
	Regions []Region
}

type Result struct {
	Report      reportv2.Report
	Diagnostics []Diagnostic
	Surface     surface.RegionSurface
}
