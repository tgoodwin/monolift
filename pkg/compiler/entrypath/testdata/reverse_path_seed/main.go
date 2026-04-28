package main

import "net/http"

func root() {}

func handler(http.ResponseWriter, *http.Request) {}

func register(http.Handler) {}

func caller() {
	register(http.HandlerFunc(handler))
	root()
}

func main() {
	caller()
}
