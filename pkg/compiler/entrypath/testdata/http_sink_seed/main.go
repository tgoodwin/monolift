package main

import "net/http"

func handler(http.ResponseWriter, *http.Request) {}

func callback() {}

func registerHTTP(http.Handler) {}

func registerCallback(func()) {}

func httpOwner() {
	registerHTTP(http.HandlerFunc(handler))
}

func callbackOwner() {
	registerCallback(callback)
}

func main() {
	httpOwner()
	callbackOwner()
}
