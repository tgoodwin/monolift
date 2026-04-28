package main

import "net/http"

func root() {}

func external(http.ResponseWriter, *http.Request) {}

func register(http.Handler) {}

func wrapper(fn func(http.ResponseWriter, *http.Request)) {
	register(http.HandlerFunc(fn))
}

func caller() {
	wrapper(external)
	root()
}

func main() {
	caller()
}
