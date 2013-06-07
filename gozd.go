// The idea is came from nginx and thie post: http://blog.nella.org/?p=879
// and code here: http://code.google.com/p/jra-go/source/browse/#hg%2Fcmd%2Fupgradable

package gozd

import (
  "net"
  "net/http"
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
)

type configGroup struct {
  name string
  mode string
  listen string // ip address or socket
}

// configuration
var (
  optSendSignal= flag.String("s", "", "Send signal to a master process: <stop, quit, reopen, reload>.")
  optConfigFile= flag.String("c", "", "Set configuration file path." )
  optHelp= flag.Bool("h", false, "This help")
  optGroups= []configGroup{}
)

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
    result :=resultByte[idxBegin:idxEnd]

    str := string(result)
    fmt.Println(str)
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
      return -1
  }

  // failure
  if ret2 < 0 {
      os.Exit(-1)
  }

  // handle exception for darwin
  if darwin && ret2 == 1 {
      ret = 0
  }

  // if we got a good PID, then we call exit the parent process.
  if ret > 0 {
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
  
   
func HandleTCPFunc(handler func(*net.Conn)) {
  
}

func HandleHTTPFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
  
}

func HandleFCGIRequest(pattern string, handler http.Handler) {
  
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
    case "reopen","reload":
      if (pid != 0) {
        p,err := os.FindProcess(pid)
        if (err == nil) {
          p.Signal(syscall.SIGTERM)
          // wait it end
          // start daemon
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
}

func signalHandler(cSignal chan os.Signal) {
  signal.Notify(cSignal, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR2, syscall.SIGINT)
  // Block until a signal is received.
  for s := range cSignal {
    fmt.Println("Got signal:", s)
    switch (s) {
      case syscall.SIGHUP, syscall.SIGUSR2:
        // upgrade, reopen
      case syscall.SIGTERM, syscall.SIGINT:
        // quit
        os.Exit(0)
    }
  }
  
}

