package bootpath

import "go/token"

type BootSpec struct {
	ConfigSources     []ConfigSource
	DependencyInits   []DependencyInit
	GoroutineLaunches []GoroutineLaunch
	Refusals          []BootPathRefusal
	EntryPath         []string
}

type ConfigSource interface {
	isConfigSource()
	Kind() string
	Identifier() string
}

type FileFormat string

const (
	FileFormatUnknown FileFormat = "unknown"
	FileFormatJSON    FileFormat = "json"
)

type EnvSource struct {
	Name      string
	Default   string
	Required  bool
	SSAOrigin token.Pos
}

type FlagSource struct {
	Name      string
	Default   string
	Required  bool
	FlagSet   string
	SSAOrigin token.Pos
}

type FileSource struct {
	Path      string
	Format    FileFormat
	MountName string
	Required  bool
	SSAOrigin token.Pos
}

type LiteralSource struct {
	Value     string
	SSAOrigin token.Pos
}

type DBSource struct {
	Name       string
	QueryShape string
	Required   bool
	SSAOrigin  token.Pos
}

type DependencyInit struct {
	Name           string
	Classification string
	SSAOrigin      token.Pos
}

const (
	DependencyRequired                = "required"
	DependencySubstitutable           = "substitutable"
	DependencyDisabledByMinimalConfig = "disabled-by-minimal-config"
)

type GoroutineLaunch struct {
	Callee    string
	SSAOrigin token.Pos
}

type BootPathRefusal struct {
	Kind      string
	Message   string
	SSAOrigin token.Pos
}

const (
	RefusalUnportableLiteralPath = "UnportableLiteralPath"
	RefusalRequiredDBSource      = "RequiredDBSource"
	RefusalUnportableDependency  = "UnportableDependency"
)

func (EnvSource) isConfigSource()     {}
func (FlagSource) isConfigSource()    {}
func (FileSource) isConfigSource()    {}
func (LiteralSource) isConfigSource() {}
func (DBSource) isConfigSource()      {}

func (s EnvSource) Kind() string     { return "env" }
func (s FlagSource) Kind() string    { return "flag" }
func (s FileSource) Kind() string    { return "file" }
func (s LiteralSource) Kind() string { return "literal" }
func (s DBSource) Kind() string      { return "db" }

func (s EnvSource) Identifier() string     { return s.Name }
func (s FlagSource) Identifier() string    { return s.Name }
func (s FileSource) Identifier() string    { return s.Path }
func (s LiteralSource) Identifier() string { return s.Value }
func (s DBSource) Identifier() string      { return s.Name }
