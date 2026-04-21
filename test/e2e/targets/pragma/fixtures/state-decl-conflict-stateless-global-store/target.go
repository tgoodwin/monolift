package statedeclconflictstatelessglobalstorefixture

var packageCounter int

//monolift:lift name=bad-stateless state=stateless
func Record(n int) error {
	packageCounter = n
	return nil
}
