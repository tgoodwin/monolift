package main

import (
	_ "example.com/rootresolution/one"
	_ "example.com/rootresolution/two"
)

type Exact struct{}

func (*Exact) Root() {}

type Bare struct{}

func (*Bare) Root() {}

func main() {}
