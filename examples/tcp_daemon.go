// Use of this source code is governed by a BSD-style license

package main

import (
  "../"
  "fmt"
  "runtime"
  "time"
  //"strings"
  "syscall"
  "strconv"
)

const (
  TCP_MAX_TRANSMISSION_BYTES = 2 * 1024 // 2KB
  TCP_CONNECTION_TIMEOUT = 12 * time.Hour
)

func serveTCP(conn gozd.GOZDConn) {
  fmt.Println("Caller serveTCP!")
  conn.SetDeadline(time.Now().Add(TCP_CONNECTION_TIMEOUT))
  defer conn.Close()
  sendCnt := 1
  selfPID := syscall.Getpid()
  for {
    respondString := "\nPID: " + strconv.Itoa(selfPID) + "\nCount: " + strconv.Itoa(sendCnt)
    respond := []byte(respondString)
    _, err := conn.Write(respond)
    sendCnt++
    if err != nil {
      fmt.Println(err.Error())
      break
    }

    time.Sleep(time.Second)
  }
}

func main() {
  runtime.GOMAXPROCS(runtime.NumCPU())
  gozd.Daemonize()
  // Listen on TCP port 13798 on all interfaces.
  l, err := gozd.Listen("tcp", ":13798")
  if err != nil {
    fmt.Println(err.Error())
    return
  }
  fmt.Println("Caller start to listen tcp port 13798")
  
  for {
    // Wait for a connection.
    conn, err := l.Accept()
    if err != nil {
      fmt.Println(err.Error())
    }
    // Handle the connection in a new goroutine.
    // The loop then returns to accepting, so that
    // multiple connections may be served concurrently.
    go serveTCP(conn)
    runtime.Gosched()
  }

  return
}
