package main

type Handler struct {
	Run func()
}

func main() {
	h := &Handler{Run: target}
	h.execute()
}

func (h *Handler) execute() {}

func target() {}
