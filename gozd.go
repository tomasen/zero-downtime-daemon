// The idea is came from nginx and thie post: http://blog.nella.org/?p=879
// and code here: http://code.google.com/p/jra-go/source/browse/#hg%2Fcmd%2Fupgradable

package gozd

import (
  "net"
  //"net/http"
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
  "time"
  "container/list"
  "sync"
)

type configGroup struct {
  name string
  mode string
  listen string // ip address or socket
}

type GOZDCounter struct {
  mutex sync.Mutex
  c int
}

type GOZDConn struct {
  element *list.Element
  net.Conn
}

// configuration
var (
  optSendSignal= flag.String("s", "", "Send signal to a master process: <stop, quit, reopen, reload>.")
  optConfigFile= flag.String("c", "", "Set configuration file path." )
  optHelp= flag.Bool("h", false, "This help")
  optGroups= []configGroup{}
  openedGOZDConns = list.New()
  openedGOZDListeners = list.New() // FDs opened by callers
  daemonListeners = make(map[string]net.Listener) // use to receive daemon command
)

const (
  TCP_REQUEST_MAX_BYTES = 2 * 1024 // 2KB
  TCP_CONNECTION_TIMEOUT = 2 * time.Minute
)

type GOZDListener struct { // used for caller, instead of default net.Listener
  element *list.Element
  net.Listener
}

func newDaemonListener(netType, laddr, name string) (net.Listener, error) {
  l, err := net.Listen(netType, laddr)
  if err != nil {
    fmt.Println(err.Error())
    return nil, err
  }
  daemonListeners[name] = l
  return l, err
}

func Listen(netType, laddr string) (GOZDListener, error) {
  l, err := net.Listen(netType, laddr)
  listenerGOZD := GOZDListener{Listener: l}
  if err != nil {
    return listenerGOZD, err
  }

  listenerGOZD.element = openedGOZDListeners.PushBack(listenerGOZD)
  return listenerGOZD, err
}

func (listener *GOZDListener) Accept() (GOZDConn, error) {
  fmt.Println("GOZDListener Accepted.")
  conn, err := listener.Listener.Accept()
  connGOZD := GOZDConn{Conn: conn}
  if err != nil {
    return connGOZD, err
  }

  // Wrap the returned connection, so that we can observe when
  // it is closed.
  connGOZD.element = openedGOZDConns.PushBack(connGOZD)
  return connGOZD, err
}

func (listener *GOZDListener) Close() {
  fmt.Println("GOZDListener Closed.")
  openedGOZDListeners.Remove(listener.element)
  listener.Listener.Close()
}

func (conn *GOZDConn) Close() error {
  fmt.Println("GOZDConn Closed.")
  openedGOZDConns.Remove(conn.element)
  return conn.Conn.Close()
}

func parseConfigFile(filePath string) bool {
  configString, err := readStringFromFile(filePath)
  if err != nil {
    fmt.Println(err.Error())
    return false
  }

  newGroup := configGroup{
    name: "",
    mode: "",
    listen: "",
  }
  optGroups = append(optGroups, newGroup)
  splitLines := strings.Split(configString, "\n")
  groupIdx := 0
  for idx := 0; idx < len(splitLines);idx++ {
    param := extractParam(splitLines[idx])
    if param == "" {
      continue
    }
    
    if strings.Contains(splitLines[idx], "mode") {
      optGroups[groupIdx].mode = param
    } else if strings.Contains(splitLines[idx], "listen") {
      optGroups[groupIdx].listen = param
    } else {
      optGroups[groupIdx].name = param
    }

    if optGroups[groupIdx].name != "" && optGroups[groupIdx].mode != "" && optGroups[groupIdx].listen != "" {
      newGroup := configGroup{
        name: "",
        mode: "",
        listen: "",
      }
      optGroups = append(optGroups, newGroup)
      groupIdx ++
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

func getPidByConf (confPath string, prefix string) (int, error) {
  
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
  
// These functions are used to handle daemon control commands & dispatch, not for outsiders
func commandDispatcherTCP(conn net.Conn) {

  conn.SetReadDeadline(time.Now().Add(TCP_CONNECTION_TIMEOUT))
  request := make([]byte, TCP_REQUEST_MAX_BYTES)
  defer conn.Close()
  
  for {
    read_len, err := conn.Read(request)
    if err != nil {
      fmt.Println(err.Error())
      break
    }

    if read_len == 0 {
      break
    }

    if strings.Contains(string(request), "DISCONNECT") {
      break // client disconnected
    }

    // TODO: Parse & Dispatch request
  }
}

// TODO
/*func commandDispatcherHTTP(pattern string, handler func(http.ResponseWriter, *http.Request)) {
  
}

func commandDispatcherFCGI(pattern string, handler http.Handler) {
  
}*/

func tcpHandler(l net.Listener) {
  for {
    // Wait for a connection.
    conn, err := l.Accept()
    if err != nil {
      log.Fatal(err)
    }
    // Handle the connection in a new goroutine.
    // The loop then returns to accepting, so that
    // multiple connections may be served concurrently.
    go commandDispatcherTCP(conn)

    runtime.Gosched()
  }
}

func Daemonize() {
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

  // start listen
  for _, val := range optGroups {
    switch val.mode {
      case "tcp":
      {
        l, err := net.Listen("tcp", val.listen)
        if err != nil {
          fmt.Println(err.Error())
          os.Exit(0)
        }
        go tcpHandler(l)
      }
      case "http":
        continue // TODO
      default:
        continue
    }
  }

  // handle signals
  // Set up channel on which to send signal notifications.
  // We must use a buffered channel or risk missing the signal
  // if we're not ready to receive when the signal is sent.
  cSignal := make(chan os.Signal, 1)
  go signalHandler(cSignal)
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

