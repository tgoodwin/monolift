package fixture

//monolift:lift name=keyed-no-mutex methods=ServeHTTP
type Handler struct {
	values map[string]int
}

func (h *Handler) ServeHTTP(key string) {
	h.values[key] = 1
	delete(h.values, key)
}
