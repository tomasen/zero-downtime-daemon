package main

import (
 "net"
 "net/http"
 "net/http/fcgi"
)

type FastCGIServer struct{}

func (s FastCGIServer) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
 resp.Write([]byte("<h1>Hello, 世界</h1>\n<p>Behold my Go web app.</p>"))
}

func main() {
  // Demonlize this
 listener, _ := net.Listen("unix", "/tmp/go.socket")
 srv := new(FastCGIServer)
 fcgi.Serve(listener, srv)
}