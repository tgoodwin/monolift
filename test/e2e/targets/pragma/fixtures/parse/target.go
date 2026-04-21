package parsefixture

import "context"

//monolift:lift name=parse-broken mode=dynamic policy="trigger=CPU threshold=0.70
func ParseBroken(ctx context.Context) error {
	return nil
}
