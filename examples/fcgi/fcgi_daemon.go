package main

import (
  //"bitbucket.org/PinIdea/go-zero-downtime-daemon"
  "../../"
  "net"
  "net/http"
  "net/http/fcgi"
)

type FastCGIServer struct{}

func (s FastCGIServer) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
  
  switch{
    case strings.HasPrefix(strings.ToLower(req.RequestURI), "/search"):
      shooter.HandleSearch(resp, req)
  }
  resp.Write([]byte("<p>unable to handler the request</p>"))
}

func main() {
  daemonChan, isSucceed := gozd.Daemonize() // returns channel that connects with daemon
  if !isSucceed {
    os.Exit(1)
  }
  
  err := gozd.RegistHandler("Group0", "ServeFCGI", serveHTTP) // regist your own handle function, parameters MUST contain a "gozd.Conn" type.
  if err != nil {
    gozd.LogErr(err.Error())
    os.Exit(1)
  }
  
  waitTillFinish(daemonChan) // wait till daemon send a exit signal
}

func waitTillFinish(daemonChan chan int) {
  code := <- daemonChan
  gozd.Log("Exit tcp_daemon.")
  os.Exit(code)
}
