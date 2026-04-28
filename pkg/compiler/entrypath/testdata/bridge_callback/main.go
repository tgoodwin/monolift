package main

import "net/http"

type API struct{}

func main() {
	var api API
	api.install()
}

func regionRoot() {}

func callback(w http.ResponseWriter, r *http.Request) {
	regionRoot()
}

func otherCallback(w http.ResponseWriter, r *http.Request) {
	regionRoot()
}

func (api *API) install() {
	api.accept(http.HandlerFunc(callback))
	_ = http.HandlerFunc(otherCallback)
}

func (api *API) accept(handler http.Handler) {
	_ = handler
}
