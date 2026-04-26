package fixture

import "sync"

//monolift:lift name=mutex-keyed-map methods=ServeHTTP
type Handler struct {
	connections   map[string]int
	connectionsMu sync.Mutex
}

func (h *Handler) ServeHTTP(key string) {
	h.connectionsMu.Lock()
	h.connections[key] = 1
	delete(h.connections, key)
	h.connectionsMu.Unlock()
}
