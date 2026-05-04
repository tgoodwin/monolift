package main

func main() {
	A()
}

func A() {}

func B() {
	var w worker = D{}
	w.Work()
}

type worker interface {
	Work()
}

type D struct{}

func (D) Work() {}
