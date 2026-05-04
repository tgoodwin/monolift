package main

type worker interface {
	Work()
}

type impl struct{}

func (*impl) Work() {}

type algorithm struct {
	worker
}

var registry = map[string]func(string) worker{}

func init() {
	mustRegister("impl", newImpl)
}

func main() {
	dispatch()
}

func mustRegister[T worker](name string, newFn func(string) T) {
	register(name, newFn)
}

func register[T worker](name string, newFn func(string) T) {
	registry[name] = func(config string) worker {
		return newFn(config)
	}
}

func dispatch() {
	fn := registry["impl"]
	_ = fn("")
}

func parse() *algorithm {
	fn := registry["impl"]
	return &algorithm{worker: fn("")}
}

func use(a *algorithm) {
	a.Work()
}

func newImpl(string) *impl {
	return &impl{}
}
