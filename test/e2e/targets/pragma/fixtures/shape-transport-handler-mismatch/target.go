package shapetransporthandlermismatchfixture

import "context"

type S struct{}

//monolift:lift name=bad-handler transport=handler
func (s *S) Compute(ctx context.Context, n int) (int, error) {
	return n + 1, nil
}
