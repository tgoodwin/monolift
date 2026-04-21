package closurewalk

import "strings"

var PackageGlobal = "global"

const PackageConst = "const"

type Handler struct{}

type Greeter interface {
	Greet() string
}

type greeterImpl struct{}

type otherGreeter struct{}

var _ Greeter = otherGreeter{}

func makeGreeter() Greeter {
	return greeterImpl{}
}

func unusedGreeter() Greeter {
	return otherGreeter{}
}

func helper(input string) string {
	return strings.ToUpper(input) + PackageConst
}

func (greeterImpl) Greet() string {
	return helper(PackageGlobal)
}

func (otherGreeter) Greet() string {
	return helper("unused")
}

func (h *Handler) ServeHTTP() string {
	prefix := "prefix"
	local := func() string {
		return helper(prefix + PackageGlobal)
	}
	return makeGreeter().Greet() + local()
}
