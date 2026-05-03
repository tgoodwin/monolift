package eval

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tgoodwin/monolift/pkg/activation"
)

// TraceFunctionKeys returns canonical function identities for all trace steps
// that name a function.
func TraceFunctionKeys(trace Trace, target Target) ([]activation.FunctionKey, error) {
	keys := make([]activation.FunctionKey, 0, len(trace.Steps))
	for _, step := range trace.Steps {
		key, err := TraceStepFunctionKey(step, target)
		if err != nil {
			return nil, fmt.Errorf("%s step %d: %w", trace.ID, step.Step, err)
		}
		if !key.IsZero() {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// TraceStepFunctionKey normalizes a JSON trace step into
// (package_path, receiver, func_name).
func TraceStepFunctionKey(step TraceStep, target Target) (activation.FunctionKey, error) {
	if step.Func == nil || strings.TrimSpace(*step.Func) == "" {
		return activation.FunctionKey{}, nil
	}
	funcName, receiver := parseTraceFunc(*step.Func)
	if funcName == "" {
		return activation.FunctionKey{}, fmt.Errorf("could not parse function %q", *step.Func)
	}
	pkgPath, err := tracePackagePath(step.To, target)
	if err != nil {
		return activation.FunctionKey{}, err
	}
	return activation.FunctionKey{
		PackagePath: pkgPath,
		Receiver:    receiver,
		FuncName:    funcName,
	}, nil
}

func parseTraceFunc(raw string) (funcName, receiver string) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.Trim(cleaned, "`")
	cleaned = strings.TrimSuffix(cleaned, "()")
	if strings.HasPrefix(cleaned, "(") {
		idx := strings.Index(cleaned, ").")
		if idx >= 0 {
			receiver = strings.TrimPrefix(cleaned[1:idx], "*")
			if strings.HasPrefix(cleaned[1:idx], "*") {
				receiver = "*" + receiver
			}
			return cleaned[idx+2:], receiver
		}
	}
	if idx := strings.LastIndex(cleaned, "."); idx >= 0 {
		return cleaned[idx+1:], cleaned[:idx]
	}
	return cleaned, ""
}

func tracePackagePath(to string, target Target) (string, error) {
	if target.ModulePath == "" {
		return "", fmt.Errorf("missing module path for target %s", target.Name)
	}
	filePath, err := traceFilePath(to, target)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(filePath)
	if dir == "." {
		return target.ModulePath, nil
	}
	return target.ModulePath + "/" + filepath.ToSlash(dir), nil
}

func traceFilePath(to string, target Target) (string, error) {
	cleaned := cleanTraceLocation(to)
	if cleaned == "" {
		return "", fmt.Errorf("missing trace location")
	}
	pathPart := cleaned
	if idx := strings.LastIndex(pathPart, ":"); idx >= 0 {
		pathPart = pathPart[:idx]
	}
	pathPart = filepath.ToSlash(pathPart)
	projectPrefix := "evaluation/" + target.Name + "/"
	if strings.HasPrefix(pathPart, projectPrefix) {
		pathPart = strings.TrimPrefix(pathPart, projectPrefix)
	}
	absProject := filepath.ToSlash(target.ProjectDir) + "/"
	if strings.HasPrefix(pathPart, absProject) {
		pathPart = strings.TrimPrefix(pathPart, absProject)
	}
	if target.Name == "mattermost" {
		pathPart = strings.TrimPrefix(pathPart, "server/")
	}
	return pathPart, nil
}

func cleanTraceLocation(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.Trim(cleaned, "`")
	if idx := strings.IndexAny(cleaned, " \t"); idx >= 0 {
		cleaned = cleaned[:idx]
	}
	return cleaned
}
