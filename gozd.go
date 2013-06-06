package gozd

import (
  "net"
  "net/http"
)

type TCPHandler interface {
  ServeTCP(*net.Conn)
}
    
func HandleTCPConnection(handler TCPHandler) {
  
}

func HandleHTTPRequest(pattern string, handler http.Handler) {
  
}

func HandleFCGIRequest(pattern string, handler http.Handler) {
  
}

func Daemonize() error {
  
  
}