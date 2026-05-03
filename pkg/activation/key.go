package activation

import "strings"

// FunctionKey is the stable identity used for comparing trace steps with SSA
// functions without depending on source line positions.
type FunctionKey struct {
	PackagePath string `json:"package_path"`
	Receiver    string `json:"receiver,omitempty"`
	FuncName    string `json:"func_name"`
}

func (k FunctionKey) String() string {
	if k.PackagePath == "" && k.Receiver == "" {
		return k.FuncName
	}
	if k.Receiver != "" {
		return k.PackagePath + "." + k.Receiver + "." + k.FuncName
	}
	return k.PackagePath + "." + k.FuncName
}

func (k FunctionKey) IsZero() bool {
	return k.PackagePath == "" && k.Receiver == "" && k.FuncName == ""
}

// Fuzzy returns a comparison key that ignores pointer receivers, SSA wrapper
// suffixes, and generic instantiation details.
func (k FunctionKey) Fuzzy() FunctionKey {
	return FunctionKey{
		PackagePath: k.PackagePath,
		Receiver:    fuzzyName(strings.TrimPrefix(k.Receiver, "*")),
		FuncName:    fuzzyName(k.FuncName),
	}
}

func fuzzyName(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.Index(name, "["); idx >= 0 {
		name = name[:idx]
	}
	if idx := strings.Index(name, "$"); idx >= 0 {
		name = name[:idx]
	}
	return name
}
