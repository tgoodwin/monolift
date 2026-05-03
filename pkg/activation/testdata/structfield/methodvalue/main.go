package main

type Handler struct {
	Run func()
}

type Worker struct{}

func main() {
	w := Worker{}
	var h Handler
	h.Run = w.Method
	dispatch(&h)
}

func dispatch(h *Handler) {
	h.Run()
}

func (Worker) Method() {}
