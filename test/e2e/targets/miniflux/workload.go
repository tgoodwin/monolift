package miniflux

import (
	"context"
	"errors"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

type Workload struct{}

func (Workload) Setup(ctx context.Context, host string) error {
	return errors.New("miniflux workload deferred until SPRINT-0005")
}

func (Workload) Action(ctx context.Context, host string) (harness.Transcript, error) {
	return harness.Transcript{}, errors.New("miniflux workload deferred until SPRINT-0005")
}

func (Workload) Verify(ctx context.Context, host string, expected harness.Transcript) error {
	return errors.New("miniflux workload deferred until SPRINT-0005")
}
