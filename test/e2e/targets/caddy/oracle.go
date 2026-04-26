package caddy

import (
	"fmt"
	"net/http"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type Oracle struct{}

func (Oracle) Invoke(args map[string]any) (any, error) {
	symbol, _ := args["symbol"].(string)
	if symbol == "" {
		symbol = "cleanpath"
	}
	switch symbol {
	case "cleanpath":
		return invokeCleanPath(args)
	case "sanitizemethod":
		return invokeSanitizeMethod(args)
	default:
		return nil, fmt.Errorf("unknown symbol %q", symbol)
	}
}

func invokeCleanPath(args map[string]any) (any, error) {
	p, ok := args["p"].(string)
	if !ok {
		return nil, fmt.Errorf("p must be string")
	}
	collapseSlashes, ok := args["collapse_slashes"].(bool)
	if !ok {
		return nil, fmt.Errorf("collapse_slashes must be bool")
	}
	return caddyhttp.CleanPath(p, collapseSlashes), nil
}

func invokeSanitizeMethod(args map[string]any) (any, error) {
	m, ok := args["m"].(string)
	if !ok {
		return nil, fmt.Errorf("m must be string")
	}
	if sanitized, ok := methodMap[m]; ok {
		return sanitized, nil
	}
	return "OTHER", nil
}

var methodMap = map[string]string{
	"GET": http.MethodGet, "get": http.MethodGet,
	"HEAD": http.MethodHead, "head": http.MethodHead,
	"PUT": http.MethodPut, "put": http.MethodPut,
	"POST": http.MethodPost, "post": http.MethodPost,
	"DELETE": http.MethodDelete, "delete": http.MethodDelete,
	"CONNECT": http.MethodConnect, "connect": http.MethodConnect,
	"OPTIONS": http.MethodOptions, "options": http.MethodOptions,
	"TRACE": http.MethodTrace, "trace": http.MethodTrace,
	"PATCH": http.MethodPatch, "patch": http.MethodPatch,
}
