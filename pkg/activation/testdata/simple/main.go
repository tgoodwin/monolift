package main

func main() {
	top()
}

func top() {
	r := runner{}
	r.concrete()
	callIface(r)
}

type runner struct{}

func (runner) concrete() {
	helper()
}

func helper() {
}

type worker interface {
	Work()
}

func callIface(w worker) {
	w.Work()
}

func (runner) Work() {
	target()
}

func target() { // activation-target
}
