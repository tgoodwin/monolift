package v1deprecatedfixture

import "context"

// @monolift trigger=CPU threshold=0.70
func LegacyAt(ctx context.Context) error {
	return nil
}

//monolift:offload metric=MEM threshold=0.80
func LegacyOffload(ctx context.Context) error {
	return nil
}
