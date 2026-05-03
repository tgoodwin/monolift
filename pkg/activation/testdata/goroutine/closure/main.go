package main

func main() {
	go func() {
		target()
	}()
}

func target() {}
