package main

import "net/http"

type shapedHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

func handler(http.ResponseWriter, *http.Request) {}

func callback() {}

func acceptsHandler(http.Handler) {}

func acceptsHandlerFunc(http.HandlerFunc) {}

func acceptsServer(*http.Server) {}

func acceptsShaped(shapedHandler) {}

func acceptsCallback(func()) {}

func handlerOwner() {
	acceptsHandler(http.HandlerFunc(handler))
}

func handlerFuncOwner(h http.HandlerFunc) {
	acceptsHandlerFunc(h)
}

func serverOwner(s *http.Server) {
	acceptsServer(s)
}

func shapeOwner(h shapedHandler) {
	acceptsShaped(h)
}

func callbackOwner() {
	acceptsCallback(callback)
}

func main() {
	handlerOwner()
	handlerFuncOwner(http.HandlerFunc(handler))
	serverOwner(&http.Server{})
	shapeOwner(http.HandlerFunc(handler))
	callbackOwner()
}
