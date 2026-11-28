package liftability

// Negate is a value-only operation with an explicit failure channel.
func Negate(input bool) (bool, error) {
	return !input, nil
}

type Point struct {
	X int
	Y int
}

// Shift is another value-only operation.
func Shift(point Point) (Point, error) {
	return Point{X: point.X + 1, Y: point.Y}, nil
}

// SetFirst mutates caller-owned backing storage through a slice value.
func SetFirst(values []int) error {
	values[0] = 1
	return nil
}

// ReturnInputAlias exposes caller-owned backing storage in its result.
func ReturnInputAlias(values []int) (*int, error) {
	return &values[0], nil
}

type Counter struct {
	N int
}

// Increment is valid only when the Counter is owned by a singleton region.
func (counter *Counter) Increment() error {
	counter.N++
	return nil
}

var externalTotal int

// AddExternal represents state owned by an external durable system in the
// abstract model; this fixture does not claim to implement that system.
func AddExternal(value int) error {
	externalTotal += value
	return nil
}

// NoErrorResult lacks the explicit failure channel required by the model.
func NoErrorResult(input int) int {
	return input + 1
}
