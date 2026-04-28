package main

import "net/http"

type handlerFunc func(http.ResponseWriter, *http.Request)

func (h handlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(w, r)
}

type server struct {
	handler http.Handler
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func newServer(h http.Handler) *server {
	s := &server{}
	s.handler = h
	return s
}

func root() {}

func external(http.ResponseWriter, *http.Request) {
	root()
}

func main() {
	_ = newServer(handlerFunc(external))
}
