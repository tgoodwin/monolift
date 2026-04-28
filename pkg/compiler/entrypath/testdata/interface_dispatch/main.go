package main

import "net/http"

type appHandler struct{}

func (appHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func register(h http.Handler) {
	_ = h
}

func main() {
	register(appHandler{})
}
