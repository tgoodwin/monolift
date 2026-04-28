package main

func root() {}

func caller() {
	root()
}

func main() {
	caller()
}
