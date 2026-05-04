package main

func main() {
	go worker()
}

func worker() {
	target()
}

func target() {}
