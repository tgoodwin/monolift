package main

import (
	"net/http"

	"boundary_frontier/helper"
)

func external(http.ResponseWriter, *http.Request) {}

func root() {
	entry()
}

func entry() {
	helper.CallRegister(http.HandlerFunc(external))
}

func main() {
	root()
}
