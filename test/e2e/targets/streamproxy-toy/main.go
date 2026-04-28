package main

import (
	"io"
	"net/http"
)

type RawEcho struct{}

//monolift:lift name=raw-echo mode=remote transport=streamproxy methods=ServeHTTP
func (RawEcho) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: raw\r\nConnection: Upgrade\r\n\r\n"))
	_, _ = io.Copy(conn, conn)
}

func main() {
	http.Handle("/ws", RawEcho{})
	_ = http.ListenAndServe(":8080", nil)
}
