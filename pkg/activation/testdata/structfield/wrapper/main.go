package main

type Handler struct {
	Run func()
}

type innerFunc func()

func main() {
	var h Handler
	h.Run = wrap(inner)
	dispatch(&h)
}

func dispatch(h *Handler) {
	h.Run()
}

func inner() {}

func wrap(f innerFunc) func() {
	return func() {
		f()
	}
}
