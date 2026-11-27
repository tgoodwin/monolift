package main

// negate is the deliberately small value-only boundary. Its error result is
// explicit even though this fixture never produces an application error.
func negate(input bool) (bool, error) {
	return !input, nil
}

func observe(input bool) {
	value, err := negate(input)
	println(input, value, err == nil)
}

func main() {
	observe(false)
	observe(true)
}
