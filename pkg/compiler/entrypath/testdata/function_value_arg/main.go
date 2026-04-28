package main

func handler() {}

func register(fn func()) {
	fn()
}

func main() {
	register(handler)
}
