package codegen

import (
	"fmt"
	"go/format"
	"sort"
	"strings"
)

type importSpec struct {
	Alias string
	Path  string
}

func formatGo(filename string, src []byte) ([]byte, error) {
	formatted, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("format %s: %w\n%s", filename, err, src)
	}
	return formatted, nil
}

func uniqueImports(imports []importSpec) []importSpec {
	seen := map[string]importSpec{}
	for _, imp := range imports {
		if imp.Path == "" {
			continue
		}
		key := imp.Alias + "\x00" + imp.Path
		seen[key] = imp
	}
	out := make([]importSpec, 0, len(seen))
	for _, imp := range seen {
		out = append(out, imp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Alias < out[j].Alias
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func importSpecFromRaw(raw string) importSpec {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return importSpec{}
	}
	if strings.HasPrefix(raw, "_ ") {
		return importSpec{Alias: "_", Path: strings.TrimSpace(strings.TrimPrefix(raw, "_ "))}
	}
	return importSpec{Path: raw}
}

func exportedFieldName(name string) string {
	switch name {
	case "baseURL":
		return "BaseURL"
	case "rawHTML":
		return "RawHTML"
	case "userID":
		return "UserID"
	case "feedID":
		return "FeedID"
	}
	if name == "" {
		return "Value"
	}
	parts := strings.Split(toSnake(name), "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func stubFuncName(plan *Plan) string {
	return plan.CutPoint.FuncName + "_monolift"
}

func zeroValue(goType string) string {
	switch goType {
	case "", "error":
		return "nil"
	case "string":
		return `""`
	case "bool":
		return "false"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "float32", "float64":
		return "0"
	default:
		if strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "map[") {
			return "nil"
		}
		return goType + "{}"
	}
}

func callArgs(params []Param, reconstructed []ReconstructedParam, statePrefix string) []string {
	total := len(params) + len(reconstructed)
	args := make([]string, total)
	for _, param := range params {
		args[param.Index] = "req." + exportedFieldName(param.Name)
	}
	for _, param := range reconstructed {
		args[param.Index] = statePrefix + exportedFieldName(param.Name)
	}
	return args
}
