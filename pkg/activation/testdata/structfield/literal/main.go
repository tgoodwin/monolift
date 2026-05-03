package main

type Handler struct {
	Run func()
}

func main() {
	h := &Handler{Run: myFunc}
	dispatch(h)
}

func dispatch(h *Handler) {
	h.Run()
}

func myFunc() {}
