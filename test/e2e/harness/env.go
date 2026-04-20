package harness

import "os"

const (
	EnvEnabled      = "MONOLIFT_E2E"
	EnvKeep         = "MONOLIFT_E2E_KEEP"
	EnvCompiler     = "MONOLIFT_COMPILER"
	EnvUpdateGolden = "MONOLIFT_E2E_UPDATE_GOLDEN"

	DefaultCompilerPath = "./bin/stubcompiler"
)

func E2EEnabled() bool {
	return os.Getenv(EnvEnabled) == "1"
}

func KeepNamespaces() bool {
	return os.Getenv(EnvKeep) == "1"
}

func CompilerPath() string {
	if path := os.Getenv(EnvCompiler); path != "" {
		return FromRepoRoot(path)
	}
	return FromRepoRoot(DefaultCompilerPath)
}

func UpdateGolden() bool {
	return os.Getenv(EnvUpdateGolden) == "1"
}
