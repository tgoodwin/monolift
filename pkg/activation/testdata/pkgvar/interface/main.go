package main

type Runner interface {
	Run()
}

type concreteRunner struct{}

func (concreteRunner) Run() {}

var runner Runner

func init() {
	runner = concreteRunner{}
}

func main() {
	dispatch()
}

func dispatch() {
	runner.Run()
}
