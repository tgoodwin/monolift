package main

type Handler struct {
	Run func()
}

func main() {
	var h Handler
	h.Run = handler
	dispatch(&h)
}

func dispatch(h *Handler) {
	h.Run()
}

func handler() {
	target()
}

func target() {}
