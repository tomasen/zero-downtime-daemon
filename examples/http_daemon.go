// +build ignore
// This example demonstrate the basic usage of gozd
// Use of this source code is governed by a BSD-style license

package main
import (
  "net/http"
  "net"
  "log"
  "os"
  "syscall"
  gozd "bitbucket.org/PinIdea/zero-downtime-daemon"
)

type HTTPServer struct{}

func (s HTTPServer) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
 resp.Write([]byte("<h1>Hello, world</h1>\n<p>Behold my Go web app.</p>"))
}


func handleListners(cl chan net.Listener) {
  
  for v := range cl {
    go func(l net.Listener) {
      handler := new(HTTPServer)
      http.Serve(l, handler)
    }(v)
  }
}

func main() {
  
  ctx  := gozd.Context{
    Hash:   "http_example",
    Logfile:os.TempDir()+"http_daemon.log", 
    Directives:map[string]gozd.Server{
      "sock":gozd.Server{
        Network:"unix",
        Address:os.TempDir() + "http_daemon.sock",
      },
      "port1":gozd.Server{
        Network:"tcp",
        Address:"127.0.0.1:8080",
      },
    },
  }
  
  cl := make(chan net.Listener,1)
  go handleListners(cl)
  sig, err := gozd.Daemonize(ctx, cl) // returns channel that connects with daemon
  if err != nil {
    log.Println("error: ", err)
    return
  }
  
  // other initializations or config setting
  
  for s := range sig  {
    switch s {
    case syscall.SIGHUP, syscall.SIGUSR2:
      // do some custom jobs while reload/hotupdate
      
    
    case syscall.SIGTERM:
      // do some clean up and exit
      return
    }
  }
}

