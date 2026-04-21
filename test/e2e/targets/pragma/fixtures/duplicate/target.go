package duplicatefixture

import "context"

type Service struct{}

//monolift:lift name=service-run mode=remote
//monolift:lift name=service-run-copy mode=remote
func (s *Service) Run(ctx context.Context) error {
	return nil
}
