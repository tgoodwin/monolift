package caddy

import (
	"fmt"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type Oracle struct{}

func (Oracle) Invoke(args map[string]any) (any, error) {
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
