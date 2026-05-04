package main

type registry struct {
	handler func()
}

var callbacks registry

func main() {
	registerHandler(myFunc)
}

func registerHandler(handler func()) {
	callbacks.handler = handler
}

func dispatch() {
	callbacks.handler()
}

func myFunc() {
	target()
}

func target() {}
