package main

import (
	"log"
	"net/http"

	// TODO: add more imports for JSON, context, etc. as methods are added

	userservice "github.com/tgoodwin/monolift/demo/monolith/userservice"
)

type serviceServer struct {
	serviceDelegate userservice.Service
}

// TODO: Add main function to instantiate serviceServer and its delegate.
// TODO: Add HTTP handler methods for each interface method.

// Main function to set up the HTTP server
func main() {

	log.Fatal(http.ListenAndServe(":8080", nil))
}
