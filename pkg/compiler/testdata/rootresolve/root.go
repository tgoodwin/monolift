package rootresolve

type Handler struct{}

func (h *Handler) ServeHTTP() {}

func (Handler) Provision() error { return nil }

type App interface {
	Run() error
}

type Embedded interface {
	Ping() error
}

type Composite interface {
	Embedded
	Run() error
}
