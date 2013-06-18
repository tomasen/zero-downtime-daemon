// Use of this source code is governed by a BSD-style license
package main
import (
  "bitbucket.org/PinIdea/go-zero-downtime-daemon"
  "fmt"
  "time"
  "syscall"
  "strconv"
  "os"
)
const (
  TCP_MAX_TRANSMISSION_BYTES = 2 * 1024 // 2KB
  TCP_CONNECTION_TIMEOUT = 12 * time.Hour
)

func serveTCP(conn gozd.Conn) {
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
  daemonChan, isSucceed := gozd.Daemonize() // returns channel that connects with daemon
  if !isSucceed {
    os.Exit(1)
  }

  err := gozd.RegistHandler("Group0", "serveTCP", serveTCP) // regist your own handle function, parameters MUST contain a "gozd.Conn" type.
  if err != nil {
    gozd.LogErr(err.Error())
    os.Exit(1)
  }
  waitTillFinish(daemonChan) // wait till daemon send a exit signal
}
func waitTillFinish(daemonChan chan int) {
  code := <- daemonChan
  os.Exit(code)
}
