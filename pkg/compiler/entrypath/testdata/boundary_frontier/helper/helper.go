package helper

import "net/http"

func Register(handler http.Handler) {
	_ = handler
}

func CallRegister(handler http.Handler) {
	Register(handler)
}
