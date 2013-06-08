// The idea is came from nginx and thie post: http://blog.nella.org/?p=879
// and code here: http://code.google.com/p/jra-go/source/browse/#hg%2Fcmd%2Fupgradable

package gozd

import (
  "net"
  "flag"
  "os"
  "fmt"
  "io"
  "os/signal"
  "io/ioutil"
  "path/filepath"
  "syscall"
  "strconv"
  "crypto/sha1"
  "runtime"
  "log"
  "strings"
  "container/list"
  "reflect"
  "errors"
)

type configGroup struct {
  mode string
  listen string // ip address or socket
}

type gozdHandler struct {
  funcName string
  val reflect.Value
  listener *gozdListener
}

type Conn struct {
  element *list.Element
  net.Conn
}

type gozdListener struct { // used for caller, instead of default net.Listener
  net.Listener
}

// configuration
var (
  optSendSignal = flag.String("s", "", "Send signal to a master process: <stop, quit, reopen, reload>.")
  optConfigFile = flag.String("c", "", "Set configuration file path." )
  optHelp = flag.Bool("h", false, "This help")
  optGroups = make(map[string]*configGroup)
)

// caller's infomation & channel
var (
  openedGOZDConns = list.New()
  registeredGOZDHandler = make(map[string]*gozdHandler) // key = group name used by specific user defined handler in config file
  mainRoutineCommChan = make(chan int, 1)
)

func newGOZDListener(netType, laddr string) (*gozdListener, error) {
  l, err := net.Listen(netType, laddr)
  l_gozd := new(gozdListener)
  l_gozd.Listener = l
  return l_gozd, err
}

// Regist callback handler
func RegistHandler(groupName, functionName string, fn interface{}) (err error) {
  v := reflect.ValueOf(fn)

  defer func(v reflect.Value) {
    if e := recover(); e != nil {
      err = errors.New("Callback: [" + groupName + "]" + functionName + "is not callable.")
      return
    }

    if _, ok := optGroups[groupName]; !ok {
      err = errors.New("Group " + groupName + "not exist!")
      return
    }

    l, errListen := newGOZDListener(optGroups[groupName].mode, optGroups[groupName].listen)
    if errListen != nil {
      err = errListen
      return
    }
    newHandler := new(gozdHandler)
    newHandler.funcName = functionName
    newHandler.val = v
    newHandler.listener = l
    registeredGOZDHandler[groupName] = newHandler
    
    go startAcceptConn(groupName, l)
  }(v)

  v.Type().NumIn()
  return
}

func startAcceptConn(groupName string, listener *gozdListener) {
  for {
    // Wait for a connection.
    conn, err := listener.Accept()
    if err != nil {
      fmt.Println(err.Error())
      return
    }
    // Handle the connection in a new goroutine.
    // The loop then returns to accepting, so that
    // multiple connections may be served concurrently.
    go callHandler(groupName, conn)

    runtime.Gosched()
  }  

}

func callHandler(groupName string, params ...interface{}) {
  if _, ok := registeredGOZDHandler[groupName]; !ok {
    fmt.Println(groupName + " does not exist.")
    return
  }

  handler := registeredGOZDHandler[groupName]
  if len(params) != handler.val.Type().NumIn() {
    fmt.Println("Invalid param of[" + groupName + "]" + handler.funcName)
    return
  }
  
  in := make([]reflect.Value, len(params))
  for k, param := range params {
    in[k] = reflect.ValueOf(param)
  }
  
  handler.val.Call(in)
}

// Override Accept() method in net.Listener interface
func (l *gozdListener) Accept() (Conn, error) {
  conn, err := l.Listener.Accept()
  conn_gozd := Conn{Conn: conn}
  if err != nil {
    return conn_gozd, err
  }

  // Wrap the returned connection, so that we can observe when
  // it is closed.
  conn_gozd.element = openedGOZDConns.PushBack(conn_gozd)
  return conn_gozd, err
}

// Override Close() method in net.Listener interface
func (l *gozdListener) Close() {
  fmt.Println("GOZDListener Closed.")
  l.Listener.Close()
  for k, v := range registeredGOZDHandler {
    if v.listener == l {
      registeredGOZDHandler[k] = nil
      break
    }
  }
}

// Override Close() method in net.Conn interface
func (c *Conn) Close() error {
  fmt.Println("GOZDConn Closed.")
  openedGOZDConns.Remove(c.element)
  return c.Conn.Close() // call net.Conn.Close()
}

func parseConfigFile(filePath string) bool {
  configString, err := readStringFromFile(filePath)
  if err != nil {
    fmt.Println(err.Error())
    return false
  }

  newGroup := new(configGroup)
  splitLines := strings.Split(configString, "\n")
  groupName := ""
  for idx := 0; idx < len(splitLines); idx++ {
    param := extractParam(splitLines[idx])
    if param == "" {
      continue
    }
    
    if strings.Contains(splitLines[idx], "mode") {
      if groupName != "" {
        newGroup.mode = param
      }
    } else if strings.Contains(splitLines[idx], "listen") {
      if groupName != "" {
        newGroup.listen = param
      }
    } else {
      groupName = param
      optGroups[groupName] = newGroup
    }

    if groupName != "" && newGroup.mode != "" && newGroup.listen != "" {
      newGroup = new(configGroup)
      groupName = ""
    }
  }
  return true
}

// extract parameter between"[]"
func extractParam(raw string) string {
  if strings.Contains(raw, "#") {
    return ""
  }

  idxBegin := strings.Index(raw, "[")
  idxEnd := strings.Index(raw, "]")
  if idxBegin < 0 || idxEnd < 0 {
    return ""
  } else {
    resultByte := []byte(raw)
    return string(resultByte[idxBegin+1:idxEnd])
  }
}

func usage() {
  fmt.Println("[command] -conf = [config file]")
  flag.PrintDefaults()
}

func readStringFromFile(filepath string) (string, error) {
  contents, err := ioutil.ReadFile(filepath)
  return string(contents), err
}

func writeStringToFile (filepath string, contents string) error {
  return ioutil.WriteFile(filepath, []byte(contents), 0x777)
}

func getPidByConf(confPath string, prefix string) (int, error) {
  
  confPath,err := filepath.Abs(confPath)
  fmt.Println("Confpath: " + confPath)
  if (err != nil) {
    return 0, err
  }
  
  hashSha1 := sha1.New()
  io.WriteString(hashSha1, confPath)
  pidFilepath := filepath.Join(os.TempDir(), fmt.Sprintf("%v_%x.pid", prefix, hashSha1.Sum(nil)))
  
  pidString, err := readStringFromFile(pidFilepath)
  fmt.Println("Filepath: " + pidFilepath)
  fmt.Println("PID string: "+ pidString)
  if (err != nil) {
    return 0, err
  }
  
  return strconv.Atoi(pidString)
}

func daemon(nochdir, noclose int) int {
  var ret, ret2 uintptr
  var err syscall.Errno

  darwin := runtime.GOOS == "darwin"

  // already a daemon
  if syscall.Getppid() == 1 {
      return 0
  }

  // fork off the parent process
  ret, ret2, err = syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
  if err != 0 {
    fmt.Println("error!"+ err.Error())
    return -1
  }

  // failure
  if ret2 < 0 {
    fmt.Println("failure!"+ string(ret2))
    os.Exit(-1)
  }

  // handle exception for darwin
  if darwin && ret2 == 1 {
      ret = 0
  }

  // if we got a good PID, then we call exit the parent process.
  if ret > 0 {
    fmt.Println("Exit parent process.")
      os.Exit(0)
  }

  /* Change the file mode mask */
  _ = syscall.Umask(0)

  // create a new SID for the child process
  s_ret, s_errno := syscall.Setsid()
  if s_errno != nil {
      log.Printf("Error: syscall.Setsid errno: %d", s_errno)
  }
  if s_ret < 0 {
      return -1
  }

  if nochdir == 0 {
      os.Chdir("/")
  }

  if noclose == 0 {
      f, e := os.OpenFile("/dev/null", os.O_RDWR, 0)
      if e == nil {
          fd := f.Fd()
          syscall.Dup2(int(fd), int(os.Stdin.Fd()))
          syscall.Dup2(int(fd), int(os.Stdout.Fd()))
          syscall.Dup2(int(fd), int(os.Stderr.Fd()))
      }
  }

  return 0
}
  
func Daemonize() (chan int) {
  // parse arguments
  flag.Parse()

  // -conf parse config
  if (!parseConfigFile(*optConfigFile)) {
    fmt.Println("Config file read error!")
    usage()
    os.Exit(1)
  }

  // find master process id by conf
  pid,err := getPidByConf(*optConfigFile, "gozerodown")
  if (err != nil) {
    pid = 0 
  }
  
  // -s send signal to the process that has same config
  switch (*optSendSignal) {
    case "stop": 
    if (pid != 0) {
      p,err := os.FindProcess(pid)
      if (err == nil) {
        p.Signal(syscall.SIGTERM)
        // wait it end
        os.Exit(0)
      }
    }
    os.Exit(0)
    
    case "start","":
      // start daemon
      fmt.Println("Start daemon!")
      /*if daemon(0, 0) != 0 {
        os.Exit(1)
      }*/
    case "reopen","reload":
      if (pid != 0) {
        p,err := os.FindProcess(pid)
        if (err == nil) {
          p.Signal(syscall.SIGTERM)
          // wait it end
          // start daemon
          if daemon(0, 0) != 0 {
            os.Exit(1)
          }
        }
      }
      os.Exit(0)
  }

  // handle signals
  // Set up channel on which to send signal notifications.
  // We must use a buffered channel or risk missing the signal
  // if we're not ready to receive when the signal is sent.
  cSignal := make(chan os.Signal, 1)
  go signalHandler(cSignal)
  mainRoutineCommChan = make(chan int, 1)
  return mainRoutineCommChan
}

func signalHandler(cSignal chan os.Signal) {
  signal.Notify(cSignal, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR2, syscall.SIGINT)
  // Block until a signal is received.
  for s := range cSignal {
    fmt.Println("Got signal:", s)
    switch (s) {
      case syscall.SIGHUP, syscall.SIGUSR2:
        // upgrade, reopen
        // using exec.Command() to start a new instance
        
      case syscall.SIGTERM, syscall.SIGINT:
        // wait all clients disconnect
        // quit
        os.Exit(0)
    }
  }
  
}

