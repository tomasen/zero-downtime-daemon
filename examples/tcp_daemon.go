// +build ignore
// This example demonstrate the basic usage of gozd
// Use of this source code is governed by a BSD-style license

package main
import (
  "time"
  "net"
  "log"
  "os"
  "fmt"
  "syscall"
  "path"
  "reflect"
  gozd "bitbucket.org/PinIdea/zero-downtime-daemon"
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
      log.Println("listener: ", reflect.ValueOf(l).Elem().FieldByName("Name").String())
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
  log.Println(os.TempDir())
  ctx  := gozd.Context{
    Hash:   "tcp_example",
    Command:"start",
    Maxfds: syscall.Rlimit{Cur:32677, Max:32677},
    User:   "www",
    Group:  "www",
    Logfile:path.Join(os.TempDir(),"tcp_daemon.log"), 
    Directives:map[string]gozd.Server{
      "sock":gozd.Server{
        Network:"unix",
        Address:path.Join(os.TempDir(), "tcp_daemon.sock"),
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

