// The idea is came from nginx and thie post: http://blog.nella.org/?p=879
// and code here: http://code.google.com/p/jra-go/source/browse/#hg%2Fcmd%2Fupgradable

package gozd

import (
  "net"
  "flag"
  "os"
  "os/exec"
  "fmt"
  "io"
  "os/signal"
  "io/ioutil"
  "path/filepath"
  "syscall"
  "strconv"
  "crypto/sha1"
  "runtime"
  "strings"
  "container/list"
  "reflect"
  "errors"
  "time"
  "bitbucket.org/kardianos/osext"
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

type openedFD struct {
  fd int
  name string
  group string
}

// configuration
var (
  optSendSignal = flag.String("s", "", "Send signal to old process: <start, stop, quit, reopen, reload>.")
  optConfigFile = flag.String("c", "", "Set configuration file path." )
  optRunForeground = flag.Bool("f", false, "Running in foreground for debug.")
  optVerbose = flag.Bool("v", false, "Show GOZD log.")
  optHelp = flag.Bool("h", false, "This help")
  optGroups = make(map[string]*configGroup)
  openedFDs = make(map[string]*openedFD) // key = group name, this ONLY records FDs opened by old process, should be empty if using "-s start"
  gozdPrefix = "gozerodown" // used for SHA1 hash, change it with different daemons
  isDaemonized = false
  runningPID = -1
)

// caller's infomation & channel
var (
  openedGOZDConns = list.New()
  registeredGOZDHandler = make(map[string]*gozdHandler) // key = group name used by specific user defined handler in config file
  mainRoutineCommChan = make(chan int, 1)
)

func newGOZDListener(netType, laddr, groupName string) (*gozdListener, error) {
  var l net.Listener
  var err error

  // find if already exists by groupname
  if openedFDs[groupName] != nil {
    Log("Listen with opened FDs: [%d][%s][%s]", openedFDs[groupName].fd, openedFDs[groupName].name, openedFDs[groupName].group)
    f := os.NewFile(uintptr(openedFDs[groupName].fd), openedFDs[groupName].name)
    l, err = net.FileListener(f)
  } else {
    l, err = net.Listen(netType, laddr)
  }

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

    l, errListen := newGOZDListener(optGroups[groupName].mode, optGroups[groupName].listen, groupName)
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
      LogErr(err.Error())
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
    LogErr(groupName + " does not exist.")
    return
  }

  handler := registeredGOZDHandler[groupName]
  if len(params) != handler.val.Type().NumIn() {
    LogErr("Invalid param of[" + groupName + "]" + handler.funcName)
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
  Log("GOZDListener Closed.")
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
  Log("GOZDConn Closed.")
  openedGOZDConns.Remove(c.element)
  return c.Conn.Close() // call net.Conn.Close()
}

func parseConfigFile(filePath string) bool {
  path, _ := filepath.Abs(filePath)
  configString, err := readStringFromFile(path)
  if err != nil {
    LogErr(err.Error())
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
  Log("[command] -conf = [config file]")
  flag.PrintDefaults()
}

func readStringFromFile(filepath string) (string, error) {
  contents, err := ioutil.ReadFile(filepath)
  Log("readStringFromFile:")
  Log(string(contents))
  return string(contents), err
}

func writeStringToFile(filepath string, contents string) error {
  return ioutil.WriteFile(filepath, []byte(contents), os.ModeAppend)
}

func truncateFile(filePath string) error {
  f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0777)
  if err != nil {
    LogErr(err.Error())
    return err
  }
  err = f.Truncate(0)
  if err != nil {
    LogErr(err.Error())
    return err
  }

  return err
}

func appendFile(filePath string, contents string) error {
  f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0777)
  if err != nil {
    return err
  }
  
  var n int
  n, err = f.Write([]byte(contents))
  f.Sync()
  f.Close()

  if err == nil && n < len(contents) {
    err = io.ErrShortWrite
  }
  
  return err
}

// Get running process info(pid, fd, etc...)
func getRunningInfoByConf(confPath string, prefix string) ([]string, error) {
  infoFilepath := getRunningInfoPathByConf(confPath, prefix)

  infoString, err := readStringFromFile(infoFilepath)
  Log("Info Filepath: " + infoFilepath)
  Log("Info string: ["+ infoString + "]")
  if (err != nil) {
    return []string{}, err
  }
  result := strings.Split(infoString, "|")
  return result, err
}

func resetRunningInfoByConf(confPath string, prefix string) {
  infoFilepath := getRunningInfoPathByConf(confPath, prefix)
  truncateFile(infoFilepath) // ignore any errors
  
  str := strconv.Itoa(os.Getpid())
  str += "|"
  err := appendFile(infoFilepath, str)
  if err != nil {
    LogErr(err.Error())
    return
  }
}

func getRunningInfoPathByConf(confPath string, prefix string) string {
  confPathAbs, err := filepath.Abs(confPath)
  if err != nil {
    LogErr(err.Error())
    return ""
  }

  hashSha1 := sha1.New()
  io.WriteString(hashSha1, confPathAbs)
  workPath, _ := filepath.Abs("")
  infoFilepath := filepath.Join(workPath, "/tmp/", fmt.Sprintf("%v_%x.gozd", prefix, hashSha1.Sum(nil)))
  syscall.Mkdir(workPath+"/tmp/", 0777)
  if err != nil && strings.Contains(err.Error(), "file exists") { // this is a "hack" solution
    LogErr(err.Error())
  }
  Log("confpath: " + confPathAbs)
  Log("Info file path: " + infoFilepath)
  return infoFilepath
}

// This function is the implementation of daemon() in standard C library.
// It forks a child process which copys itself, then terminate itself to make child's PPID = 1
// For more detail, run "man 3 daemon"
func daemon(nochdir, noclose int) int {
  var ret, ret2 uintptr
  var err syscall.Errno

  // If running at foreground, don't fork and continue.
  // This is useful for development & debug
  if *optRunForeground == true {
    return 0
  }

  darwin := runtime.GOOS == "darwin"

  // already a daemon
  if syscall.Getppid() == 1 {
    return 0
  }

  // fork off the parent process
  ret, ret2, err = syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
  if err != 0 {
    LogErr(err.Error())
    return -1
  }

  pid_after_fork := strconv.Itoa(syscall.Getpid())
  ppid_after_fork := strconv.Itoa(syscall.Getppid())
  Log("PID after fork: " + pid_after_fork)
  Log("PPID after fork: " + ppid_after_fork)

  // failure
  if ret2 < 0 {
    LogErr("failure!"+ string(ret2))
    os.Exit(-1)
  }

  // handle exception for darwin
  if darwin && ret2 == 1 {
      ret = 0
  }

  // if we got a good PID, then we call exit the parent process.
  if ret > 0 {
    Log("Exit parent process PID: " + pid_after_fork)
    os.Exit(0)
  }

  Log("PID:" + pid_after_fork + " after parent process exited: " + strconv.Itoa(syscall.Getpid()))
  Log("PPID:" + ppid_after_fork + " after parent process exited: " + strconv.Itoa(syscall.Getppid()))

  /* Change the file mode mask */
  _ = syscall.Umask(0)

  // create a new SID for the child process
  s_ret, s_errno := syscall.Setsid()
  if s_errno != nil {
      LogErr("Error: syscall.Setsid errno: %d", s_errno)
  }
  if s_ret < 0 {
      return -1
  }

  if nochdir == 0 {
      os.Chdir("/")
  }

  if noclose == 0 && *optRunForeground == false {
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
  
func Daemonize() (c chan int, isSucceed bool) {
  runningPID = os.Getpid()

  if isDaemonized {
    LogErr("Daemon already daemonized.")
    return c, false
  }

  // parse arguments
  flag.Parse()

  // -conf parse config
  if (!parseConfigFile(*optConfigFile)) {
    LogErr("Config file read error!")
    usage()
    os.Exit(1)
  }

  // find master process id by conf
  infos, err := getRunningInfoByConf(*optConfigFile, gozdPrefix)
  var pid int
  infoCnt := len(infos)
  if (err != nil || infoCnt < 1) {
    pid = 0
  } else {
    pid, _ = strconv.Atoi(infos[0])
  }

  // Info count should be groupCount * 3 + 1(for PID) + 1
  // (for strings.Split() will split 1 additional element at the end)
  if infoCnt % 3 == 2 && infoCnt >= 5 {
    for i := 1; i < infoCnt - 1; i += 3 {
      fd, _ := strconv.Atoi(infos[i])
      openFD := new(openedFD)
      openFD.fd = fd
      openFD.name = infos[i+1]
      group := infos[i+2]
      openFD.group = group
      openedFDs[group] = openFD
    }
  }
  
  // send signal to the process that has same config
  switch (*optSendSignal) {
    case "stop":
    if (pid != 0) {
      isRunning := IsProcessRunning(pid)
      if (isRunning) {
        p := GetRunningProcess(pid)
        p.Signal(syscall.SIGTERM)
        os.Exit(0)
      }
    }
    os.Exit(0)
    
    case "start", "":
      if (pid != 0) {
        isRunning := IsProcessRunning(pid)
        if isRunning {
          LogErr("Daemon already started.")
          os.Exit(1)
        }
      }
      startDaemon()
    case "reopen","reload":
      // find old process, send SIGTERM then exit self
      // the 'real' new process running later starts by old process received SIGTERM
      if (pid != 0) {
        isRunning := IsProcessRunning(pid)
        if isRunning {
          p := GetRunningProcess(pid)
          p.Signal(syscall.SIGTERM)
        }
      }
      startDaemon()
  }

  // Handle OS signals
  // Set up channel on which to send signal notifications.
  // We must use a buffered channel or risk missing the signal
  // if we're not ready to receive when the signal is sent.
  cSignal := make(chan os.Signal, 1)
  go signalHandler(cSignal)
  c = mainRoutineCommChan
  isDaemonized = true
  return c, true
}

func startDaemon() {
  Log("Start daemon!")
  if daemon(0, 0) != 0 {
    os.Exit(1) 
  }
  pid := os.Getpid()
  resetRunningInfoByConf(*optConfigFile, gozdPrefix)
  path := getRunningInfoPathByConf(*optConfigFile, gozdPrefix)
  writeStringToFile(path, strconv.Itoa(pid))
}

func signalHandler(cSignal chan os.Signal) {
  signal.Notify(cSignal, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR2, syscall.SIGINT)
  // Block until a signal is received.
  for s := range cSignal {
    Log("Got signal: %v", s)
    switch (s) {
      case syscall.SIGHUP, syscall.SIGUSR2:
        // upgrade, reopen
        // 1. write running process FDs into info config(*.gozd) before starting
        // 2. stop port listening
        // 3. using exec.Command() to start a new instance
        Log("Action: PREPARE TO STOP")
        dupNetFDs()
        stopListening()
        startNewInstance("reopen")
      case syscall.SIGTERM, syscall.SIGINT:
        Log("Action: CLOSE")
        // wait all clients disconnect
        c := make(chan int , 1)
        go waitTillAllConnClosed(c)
        <- c
        // quit, send signal to let caller do cleanups
        mainRoutineCommChan <- 0
    }
  }
  
}

// A work-around solution since os.Findprocess() not working for now.
func IsProcessRunning(pid int) bool {
  if err := syscall.Kill(pid, 0); err != nil {
    return false
  } else {
    return true
  }
}

// A work-around solution to make a new os.Process since os.newProcess() is not public.
func GetRunningProcess(pid int) (p *os.Process) {
  p, _ = os.FindProcess(pid)
  return p
}

func startNewInstance(actionToOldProcess string) {
  path, _ := osext.Executable()
  args := make([]string, 0)
  args = append(args, fmt.Sprintf("-s=%s", actionToOldProcess))
  args = append(args, fmt.Sprintf("-c=%s", *optConfigFile))
  if *optRunForeground == true {
    args = append(args, "-f")
  }

  if *optVerbose == true {
    args = append(args, "-v")
  }

  cmd := exec.Command(path, args...)
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr
  err := cmd.Start()
  
  if err != nil {
    LogErr(err.Error())
  }
}

// Dup old process's fd to new one, write FD & name into info file.
func dupNetFDs() {
  resetRunningInfoByConf(*optConfigFile, gozdPrefix)
  for k, v := range registeredGOZDHandler {
    Log(k + "|%v", v)
    l := v.listener.Listener.(*net.TCPListener) // TODO: Support to net.UnixListener
    newFD, err := l.File() // net.Listener.File() call dup() to return a new FD
    if err == nil {
      fd := newFD.Fd()
      noCloseOnExec(int(fd))
      name := newFD.Name()
      Log("New fd: " + strconv.Itoa(int(fd)) + " Name: " + name + " Group: " + k)
      infoPath := getRunningInfoPathByConf(*optConfigFile, gozdPrefix)
      appendStr := strconv.Itoa(int(fd)) + "|" + name + "|" + k + "|"
      appendFile(infoPath, appendStr)
    } else {
      LogErr(err.Error())
    }
  }
}

func noCloseOnExec(fd int) {
  flag, _ := fcntl(int(fd), syscall.F_GETFD, 0)
  // clear FD_CLOEXEC bit, read man 2 fcntl for details
  flag = flag >> 1
  flag = flag << 1
  fcntl(int(fd), syscall.F_SETFD, flag)
}

// These are here because there is no API in syscall for turning OFF
// close-on-exec (yet).
func fcntl(fd int, cmd int, arg int) (val int, err error) {
  if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
    LogErr("Function fcntl has not been tested on other platforms than linux & darwin.")
  }

  r0, _, e1 := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(cmd), uintptr(arg))
  val = int(r0)
  if e1 != 0 {
    err = e1
  }
  return
}

func stopListening() {
  for _, v := range registeredGOZDHandler {
    l := v.listener.Listener
    err := l.Close()
    if err != nil {
      LogErr(err.Error())
    }
  }
}

func waitTillAllConnClosed(c chan int) {
  for openedGOZDConns.Len() != 0 {
    time.Sleep(1 * time.Second)
  }
  c <- 1
}

func Log(format string, args ...interface{}) {
  if (*optVerbose == false) {
    return
  }

  now := time.Now()
  year, month, day := now.Date()
  hour, minute, second := now.Clock()
  time_str := fmt.Sprintf("[GOZD][%d-%d-%d %d:%d:%d]", year, month, day, hour, minute, second)
  
  var nameStr string
  pidStr := fmt.Sprintf("[%d]", runningPID)
  pc, _, _, _ := runtime.Caller(1)
  name := runtime.FuncForPC(pc).Name()
  names := strings.Split(name, ".")
  if len(names) > 0 {
    nameStr = names[len(names)-1]
  }
  callerStr := "[" + nameStr + "] "
  fmt.Printf(time_str + pidStr + callerStr + format + "\n", args...)
}

func LogErr(format string, args ...interface{}) {
  now := time.Now()
  year, month, day := now.Date()
  hour, minute, second := now.Clock()
  time_str := fmt.Sprintf("[GOZDERR][%d-%d-%d %d:%d:%d]", year, month, day, hour, minute, second)
  
  var fileStr, nameStr string
  pidStr := fmt.Sprintf("[%d]", runningPID)
  pc, _, _, _ := runtime.Caller(1)
  file, line := runtime.FuncForPC(pc).FileLine(pc)
  files := strings.Split(file, "/")
  if len(files) > 0 {
    fileStr = files[len(files)-1]
  }
  name := runtime.FuncForPC(pc).Name()
  names := strings.Split(name, ".")
  if len(names) > 0 {
    nameStr = names[len(names)-1]
  }
  callerStr := "[" + nameStr + " " + fileStr + ":" + strconv.Itoa(line) + "] "
  fmt.Printf(time_str + pidStr + callerStr + format + "\n", args...)
}
