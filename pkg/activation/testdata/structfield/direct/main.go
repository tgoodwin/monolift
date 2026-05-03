package main

type Handler struct {
	Run func()
}

func main() {
	var h Handler
	h.Run = myFunc
	dispatch(&h)
}

func dispatch(h *Handler) {
	h.Run()
}

func myFunc() {}
