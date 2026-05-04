package main

var handler func()

func init() {
	handler = target
}

func main() {
	dispatch()
}

func dispatch() {
	handler()
}

func target() {}
