package fixture

import "sync"

//monolift:lift name=mutex-only-store methods=ServeHTTP
type Handler struct {
	value int
	mu    sync.Mutex
}

func (h *Handler) ServeHTTP(value int) {
	h.mu.Lock()
	h.value = value
	h.mu.Unlock()
}
