package compiler

import "fmt"

type KeyRule struct {
	Allowed        bool
	Required       bool
	ValueValidator func(string) bool
}

var surfaceKeyRules = map[Surface]map[string]KeyRule{
	SurfaceInterface: {
		"name":      {Allowed: true, Required: true},
		"mode":      {Allowed: true, ValueValidator: oneOf("local", "remote", "dynamic")},
		"state":     {Allowed: true, ValueValidator: oneOf("stateless", "singleton", "affinity", "external")},
		"transport": {Allowed: true, ValueValidator: oneOf("http-json", "handler", "grpc")},
		"impl":      {Allowed: true},
		"registry":  {Allowed: true},
		"methods":   {Allowed: true},
		"policy":    {Allowed: true},
		"dispatch":  {Allowed: true, ValueValidator: oneOf("impl", "lift-point")},
		"affinity":  {Allowed: true},
	},
	SurfaceFunction: {
		"name":      {Allowed: true, Required: true},
		"mode":      {Allowed: true, ValueValidator: oneOf("local", "remote", "dynamic")},
		"state":     {Allowed: true, ValueValidator: oneOf("stateless", "singleton", "affinity", "external")},
		"transport": {Allowed: true, ValueValidator: oneOf("http-json", "handler", "grpc")},
		"policy":    {Allowed: true},
		"affinity":  {Allowed: true},
	},
	SurfaceMethod: {
		"name":      {Allowed: true, Required: true},
		"mode":      {Allowed: true, ValueValidator: oneOf("local", "remote", "dynamic")},
		"state":     {Allowed: true, ValueValidator: oneOf("stateless", "singleton", "affinity", "external")},
		"transport": {Allowed: true, ValueValidator: oneOf("http-json", "handler", "grpc")},
		"policy":    {Allowed: true},
		"affinity":  {Allowed: true},
	},
	SurfaceStruct: {
		"name":      {Allowed: true, Required: true},
		"mode":      {Allowed: true, ValueValidator: oneOf("local", "remote", "dynamic")},
		"state":     {Allowed: true, ValueValidator: oneOf("stateless", "singleton", "affinity", "external")},
		"transport": {Allowed: true, ValueValidator: oneOf("http-json", "handler", "grpc")},
		"registry":  {Allowed: true},
		"methods":   {Allowed: true},
		"policy":    {Allowed: true},
		"affinity":  {Allowed: true},
	},
}

// site:begin pragma-v2-validator
func validatePragma(pragma *Pragma) []Diagnostic {
	if pragma == nil {
		return nil
	}
	rules := surfaceKeyRules[pragma.Surface]
	if rules == nil {
		return []Diagnostic{{
			Code:     CodeParse,
			Severity: SeverityError,
			Message:  fmt.Sprintf("unsupported pragma surface %q", pragma.Surface),
			Span:     pragma.Span,
		}}
	}

	var diagnostics []Diagnostic
	for key, value := range pragma.Options {
		if isExtensionKey(key) {
			continue
		}
		rule, knownOnSurface := rules[key]
		if !isKnownKey(key) {
			diagnostics = append(diagnostics, Diagnostic{
				Code:     CodeUnknownKey,
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown pragma key %q", key),
				Span:     pragma.Span,
			})
			continue
		}
		if !knownOnSurface || !rule.Allowed {
			diagnostics = append(diagnostics, Diagnostic{
				Code:     CodeInvalidKeyForSurface,
				Severity: SeverityError,
				Message:  fmt.Sprintf("pragma key %q is invalid for %s surface", key, pragma.Surface),
				Span:     pragma.Span,
			})
			continue
		}
		if rule.ValueValidator != nil && !rule.ValueValidator(value) {
			diagnostics = append(diagnostics, Diagnostic{
				Code:     CodeParse,
				Severity: SeverityError,
				Message:  fmt.Sprintf("malformed value %q for pragma key %q", value, key),
				Span:     pragma.Span,
			})
		}
	}
	// site:end pragma-v2-validator

	for key, rule := range rules {
		if rule.Required && pragma.Options[key] == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Code:     CodeInvalidKeyForSurface,
				Severity: SeverityError,
				Message:  fmt.Sprintf("missing required pragma key %q for %s surface", key, pragma.Surface),
				Span:     pragma.Span,
			})
		}
	}
	if pragma.Options["mode"] == "dynamic" && pragma.Options["policy"] == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Code:     CodeInvalidKeyForSurface,
			Severity: SeverityError,
			Message:  "missing required pragma key \"policy\" for mode=dynamic",
			Span:     pragma.Span,
		})
	}
	return diagnostics
}

func isExtensionKey(key string) bool {
	return key == "x-" || len(key) > 2 && key[:2] == "x-"
}

func isKnownKey(key string) bool {
	for _, rules := range surfaceKeyRules {
		if _, ok := rules[key]; ok {
			return true
		}
	}
	return false
}

func oneOf(values ...string) func(string) bool {
	allowed := map[string]bool{}
	for _, value := range values {
		allowed[value] = true
	}
	return func(value string) bool {
		return allowed[value]
	}
}
