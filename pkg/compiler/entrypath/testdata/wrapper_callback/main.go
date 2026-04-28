package main

import "net/http"

type handlerFunc func(http.ResponseWriter, *http.Request)

func (h handlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(w, r)
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func register(h http.Handler) {
	_ = h
}

func root() {}

func external(http.ResponseWriter, *http.Request) {
	root()
}

func main() {
	register(middleware(handlerFunc(external)))
}
