// +build ignore
// This example demonstrate the usage of argument flags and conf file
// after build start the process by run `args_n_conf -c ./sample.conf`
// and use `args_n_conf -c ./sample.conf -s reload` to upgrade binary or reload config
// Use of this source code is governed by a BSD-style license

package main
import (
  "time"
  "net"
  "log"
  "os"
  "flag"
  "fmt"
  "syscall"
  "io/ioutil"
  "encoding/json"
  gozd "bitbucket.org/PinIdea/zero-downtime-daemon"
)

var (
  optCommand  = flag.String("s","","send signal to a master process: stop, quit, reopen, reload")
  optConfPath = flag.String("c","","set configuration file" )
  optPidPath  = flag.String("pid","","set pid file" )
  optHelp     = flag.Bool("h",false,"this help")
)

func usage() {
  fmt.Println("[command] -conf=[config file]")
  flag.PrintDefaults()
}

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
  // parse arguments
  flag.Parse()
  
  if (*optHelp) {
    usage()
    return
  }

  // parse conf file
  file, err := ioutil.ReadFile(*optConfPath)
  if err != nil {
    log.Println("error: ", err)
    return
  }
  
  var ctx gozd.Context
  err = json.Unmarshal(file, &ctx)
  if err != nil {
    log.Println("error: ", err)
    return
  }
  
  ctx.Command = *optCommand
  ctx.Hash    = *optConfPath
  ctx.Pidfile = *optPidPath
  
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

