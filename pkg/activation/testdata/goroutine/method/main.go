package main

type Worker struct{}

func main() {
	w := Worker{}
	go w.Run()
}

func (Worker) Run() {}
