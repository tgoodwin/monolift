package top

import (
	"example.com/diamond/mid1"
	"example.com/diamond/mid2"
)

func Top() string { return mid1.Mid1() + mid2.Mid2() }
