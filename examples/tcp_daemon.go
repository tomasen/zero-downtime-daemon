// Use of this source code is governed by a BSD-style license
package main
import (
  "time"
  "net"
  "log"
  "os"
  "fmt"
  "../"
)

func serveTCP(conn net.Conn) {
  // !important: must conn.Close to release the conn from wait group
  defer conn.Close() 
  
  for {
    _, err := conn.Write([]byte(fmt.Sprintf("%d ",os.Getpid())))
    if err != nil {
      return
    }
    time.Sleep(1*time.Second)
  }
}

func handleListners(cl chan net.Listener) {
  
  for v := range cl {
    go func(l net.Listener) {
      for {
      	conn, err := l.Accept()
      	if err != nil {
      		// gozd.ErrorAlreadyStopped may occur when shutdown/reload
          log.Println("accept error: ", err)
      		break
      	}
 
      	go serveTCP(conn)
      }
    }(v)
  }
}

func main() {
  ctx  := gozd.Context{
            Hash:   "tcp_example",
            Command:"start",
            Maxfds: 32767,
            User:   "www",
            Group:  "www",
            Logfile:os.TempDir()+"tcp_daemon.log", 
            Directives:map[string]gozd.Server{
              "sock":gozd.Server{
                Network:"unix",
                Address:os.TempDir() + "tcp_daemon.sock",
              },
              "port1":gozd.Server{
                Network:"tcp",
                Address:"127.0.0.1:2133",
              },
              "port2":gozd.Server{
                Network:"tcp",
                Address:"127.0.0.1:2233",
              },
            },
          }
  
  cl := make(chan net.Listener,1)
  go handleListners(cl)
  done, err := gozd.Daemonize(ctx, cl) // returns channel that connects with daemon
  if err != nil {
    log.Println("error: ", err)
    return
  }
  
  if <- done {
    // do some clean up and exit
    
  }
}

