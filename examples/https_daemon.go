// +build ignore
// This example demonstrate the basic usage of gozd
// Use of this source code is governed by a BSD-style license

package main
import (
  "crypto/tls"
  "net/http"
  "net"
  "log"
  "os"
  "syscall"
  gozd "bitbucket.org/PinIdea/zero-downtime-daemon"
)

const (
  certFile  = "./ssl.cert"
  keyFile   = "./ssl.key"
)

type HTTPServer struct{}

func (s HTTPServer) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
 resp.Write([]byte("<h1>Hello, world</h1>\n<p>Behold my Go web app.</p>"))
}


func handleListners(cl chan net.Listener) {
  
  for v := range cl {
    go func(l net.Listener) {
      
      handler := new(HTTPServer)
      
      srv := &http.Server{Handler: handler}
      
      config := &tls.Config{}
    	if srv.TLSConfig != nil {
    		*config = *srv.TLSConfig
    	}
    	if config.NextProtos == nil {
    		config.NextProtos = []string{"http/1.1"}
    	}

    	var err error
    	config.Certificates = make([]tls.Certificate, 1)
    	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
    	if err != nil {
        log.Println("ssl cert has problem:", err)
    		return 
    	}

    	tlsListener := tls.NewListener(l, config)
      srv.Serve(tlsListener)
    }(v)
  }
}

func main() {
  
  ctx  := gozd.Context{
    Hash:   "https_example",
    Logfile:os.TempDir()+"https_daemon.log", 
    Directives:map[string]gozd.Server{
      "sock":gozd.Server{
        Network:"unix",
        Address:os.TempDir() + "https_daemon.sock",
      },
      "port1":gozd.Server{
        Network:"tcp",
        Address:"127.0.0.1:8443",
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

