package fixtures

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"sync"
)

type Request struct {
	Name string
}

type Response struct {
	Name string
}

type Item struct {
	Name string
}

type SyncReq struct {
	Mu sync.Mutex
}

type Money struct {
	Cents int64
}

func (Money) MarshalJSON() ([]byte, error) { return json.Marshal(1) }
func (*Money) UnmarshalJSON([]byte) error  { return nil }

type Hook interface {
	Run(context.Context) error
}

type Service struct {
	Name string
}

type Builder struct {
	Name string
}

var GlobalCounter int
var GlobalItem *Item
var Work = make(chan int)

func ContextFirst(ctx context.Context, req Request) error { return nil }

func NoContext(name string) error { return nil }

func NoParams() error { return nil }

func Variadic(ctx context.Context, args ...string) error { return nil }

func Callable(ctx context.Context, fn func() error) error { return fn() }

func Streaming(ctx context.Context, jobs <-chan int) error { return nil }

func SyncPrimitive(ctx context.Context, req SyncReq) error { return nil }

func Generic[T any](ctx context.Context, v T) error { return nil }

func CustomJSON(ctx context.Context, value Money) error { return nil }

func CustomJSONHold(ctx context.Context, value Money) {}

func SerializableUnknown(ctx context.Context, value any) error { return nil }

func RequestReply(ctx context.Context, req Request) (Response, error) { return Response{}, nil }

func NoError(ctx context.Context, req Request) Response { return Response{} }

func PanicOnly(ctx context.Context) Response {
	panic("boom")
}

func PanicWithError(ctx context.Context) error {
	panic("boom")
}

func ParamMutate(ctx context.Context, item *Item) error {
	item.Name = "changed"
	return nil
}

func ParamEscape(ctx context.Context, item *Item) error {
	GlobalItem = item
	return nil
}

func GlobalWrite(ctx context.Context) error {
	GlobalCounter++
	return nil
}

func GlobalRead(ctx context.Context) error {
	_ = GlobalCounter
	return nil
}

func InterfaceCallback(ctx context.Context, hook Hook) error {
	return hook.Run(ctx)
}

func ReflectCall(ctx context.Context, fn any) error {
	reflect.ValueOf(fn).Call(nil)
	return nil
}

func OSWrite(ctx context.Context) error {
	return os.WriteFile("fixture.tmp", nil, 0o644)
}

func AsyncFork(ctx context.Context) error {
	go func() {}()
	return nil
}

func LongRunning(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-Work:
		}
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {}

func (s *Service) ReadOnly(ctx context.Context) error {
	return nil
}

func (s *Service) Mutate(ctx context.Context) error {
	s.Name = "changed"
	return nil
}

func (b *Builder) WithName(name string) *Builder {
	b.Name = name
	return b
}
