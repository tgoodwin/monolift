package main

var registry = map[string]func(){}

func init() {
	registry["key"] = myFunc
}

func main() {
	dispatch()
}

func dispatch() {
	fn := registry["key"]
	fn()
}

func myFunc() {
	target()
}

func target() {}
