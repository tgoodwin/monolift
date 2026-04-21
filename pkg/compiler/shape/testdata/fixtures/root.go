package fixtures

import (
	"context"
	"net/http"
)

type HTTPHandler struct{}

func (HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

func RawHandler(w http.ResponseWriter, r *http.Request) {}

type BadServe struct{}

func (BadServe) ServeHTTP() int { return 0 }

type Builder struct{}

func (b *Builder) WithValue(n int) *Builder { return b }

type MixedSurface struct{}

func (MixedSurface) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

func (MixedSurface) Compute(ctx context.Context, req int) (int, error) { return req, nil }

type DomainRoot struct{}

func (DomainRoot) First(ctx context.Context, req int) (int, error) { return req, nil }

func (DomainRoot) Second(ctx context.Context, left int, right string) (int, error) { return left, nil }

func RequestReply(ctx context.Context, req int) (int, error) { return req, nil }

func BadRequestReply(ctx context.Context, req int) (int, string) { return req, "" }

func ManyArgs(ctx context.Context, left int, right string) (int, error) { return left, nil }

func ErrorOnly(ctx context.Context, req int) error { return nil }

func EmptyReturn(ctx context.Context, req int) {}

var internalQueue = make(chan int)

func Consume() {
	for {
		<-internalQueue
	}
}

func Unsupported(ch chan int) error { return nil }
