package sample

type Greeter interface {
	Greet() string
}

type hello struct{}

func Use() string {
	return New().Greet() + ":" + BuildTagged() + ":" + CgoMode()
}

func New() Greeter {
	return hello{}
}

func (hello) Greet() string {
	return "hi"
}
