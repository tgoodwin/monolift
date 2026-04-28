package main

import (
	"flag"
	"io"
	"net/http"
	"os"
)

type Alpha struct {
	send chan []byte
}

type Beta struct {
	send chan []byte
}

var (
	alphaEnv = os.Getenv("STREAMPROXY_ALPHA")
	betaEnv  = os.Getenv("STREAMPROXY_BETA")
	config   = flag.String("config", "config.json", "")
)

//monolift:lift name=streamproxy-multiroot mode=remote transport=streamproxy methods=ServeAlpha
func (a *Alpha) ServeAlpha(w http.ResponseWriter, r *http.Request) {
	_ = alphaEnv
	serveRaw(w, r, a.send)
}

//monolift:lift name=streamproxy-multiroot mode=remote transport=streamproxy methods=ServeBeta
func (b *Beta) ServeBeta(w http.ResponseWriter, r *http.Request) {
	_ = betaEnv
	_ = *config
	serveRaw(w, r, b.send)
}

func serveRaw(w http.ResponseWriter, r *http.Request, ch chan []byte) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: raw\r\nConnection: Upgrade\r\n\r\n"))
	_, _ = io.Copy(conn, conn)
	_ = ch
}

func main() {
	http.HandleFunc("/alpha", (&Alpha{send: make(chan []byte, 1)}).ServeAlpha)
	http.HandleFunc("/beta", (&Beta{send: make(chan []byte, 1)}).ServeBeta)
	_ = http.ListenAndServe(":8080", nil)
}
