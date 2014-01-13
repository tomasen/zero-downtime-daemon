/*
* Copyright (c) 2013, PinIdea Co. Ltd.
* Tomasen <tomasen@gmail.com> & Reck Hou <reckhou@gmail.com>
* All rights reserved.
* Redistribution and use in source and binary forms, with or without
* modification, are permitted provided that the following conditions are met:
*
*     * Redistributions of source code must retain the above copyright
*       notice, this list of conditions and the following disclaimer.
*     * Redistributions in binary form must reproduce the above copyright
*       notice, this list of conditions and the following disclaimer in the
*       documentation and/or other materials provided with the distribution.
*
* THIS SOFTWARE IS PROVIDED BY THE REGENTS AND CONTRIBUTORS "AS IS" AND ANY
* EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
* WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
* DISCLAIMED. IN NO EVENT SHALL THE COMPANY AND CONTRIBUTORS BE LIABLE FOR ANY
* DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
* (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
* LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
* ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
* (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
* SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

/* The idea is came from nginx and this post: http://blog.nella.org/?p=879
* and code here: http://code.google.com/p/jra-go/source/browse/#hg%2Fcmd%2Fupgradable
* explain here: http://stackoverflow.com/questions/5345365/how-can-nginx-be-upgraded-without-dropping-any-requests
*/

package gozd

import (
  "os"
  "io"
  "fmt"
  "net"
  "log"
  "sync"
  "errors"
  "unsafe"
  "strconv"
  "reflect"
  "syscall"
  "runtime"
  "io/ioutil"
  "os/user"
  "os/signal"
  "crypto/sha1"
  "encoding/json"
  "path"
  "path/filepath"
  "os/exec"
  osext  "bitbucket.org/PinIdea/osext"
)

var (
  cx_       chan os.Signal = make(chan os.Signal,1)
  wg_       sync.WaitGroup
  hash_     string
  confs_    map[string]Server = make(map[string]Server)
  pidfile_  string
  logfile_  *os.File
  execpath_ string
)

// https://codereview.appspot.com/7392048/#ps1002
func findProcess(pid int) (p *os.Process, err error) {
  if e := syscall.Kill(pid, syscall.Signal(0)); pid <= 0 || e != nil {
    return nil, fmt.Errorf("find process %v", e)
  }
  p = &os.Process{Pid: pid}
  runtime.SetFinalizer(p, (*os.Process).Release)
  return p, nil
}

func infopath() string {
  h := sha1.New()
  io.WriteString(h, hash_)
  return path.Join(os.TempDir(), fmt.Sprintf("gozd%x.json", h.Sum(nil)))
}

func abdicate() {
  // i'm not master anymore
  os.Remove(infopath())
}

func masterproc() (p *os.Process, err error) {
  file, err := ioutil.ReadFile(infopath())
  if err != nil {
    return
  }

  var pid int
  err = json.Unmarshal(file, &pid)
  if err != nil {
    return
  }
  
  p, err = findProcess(pid)
  return 
}

func writepid() (err error) {
  
  var p = os.Getpid()
  
  b, err := json.Marshal(p)
  if err != nil {
    return
  }
  
  if len(pidfile_) > 0 {
    _ = ioutil.WriteFile(pidfile_, b, 0666)
  }
  
  err = ioutil.WriteFile(infopath(), b, 0666)
  return
}

func setrlimit(rl syscall.Rlimit) (err error) {
  if rl.Cur > 0 && rl.Max > 0 {
    var lim syscall.Rlimit
    if err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
      log.Println("failed to get NOFILE rlimit: ", err)
      return
    }
    
    lim.Cur = rl.Cur
    lim.Max = rl.Max
    
    if err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
      log.Println("failed to set NOFILE rlimit: ", err)
      return
    }
  }
  return
}

func setuid(u string, g string) (err error) {
  
  if (len(u) <= 0) {
    return
  }
  
  uid := -1
  gid := -1
  
  for {
    userent, err := user.Lookup(u);
    if err != nil {
      if userent, err = user.LookupId(u); err != nil {
        log.Println("Unable to find user", u, err)
        break
      }
    }
  
    uid, err = strconv.Atoi(userent.Uid)
    if err != nil {
      log.Println("Invalid uid:", userent.Uid)
    }
    gid, err = strconv.Atoi(userent.Gid)
    if err != nil {
      log.Println("Invalid gid:", userent.Gid)
    }
    break
  }
  
  if uid < 0 {
    uid, err = strconv.Atoi(u)
    if err != nil {
      log.Println("Invalid uid:", u, err)
      return
    }
  }
  
  if gid < 0 {
    gid, err = strconv.Atoi(g)
    if err != nil {
      log.Println("Invalid gid:", g, err)
      return
    }
  }
  
  if err = syscall.Setgid(gid); err != nil {
    log.Println("setgid failed: ", err)
  }
  if err = syscall.Setuid(uid); err != nil {
    log.Println("setuid failed: ", err)
  }
  return
}

// distinguish master/worker/shuting down process
// see: http://stackoverflow.com/questions/14926020/setting-process-name-as-seen-by-ps-in-go
func setProcessName(name string) error {
  argv0str := (*reflect.StringHeader)(unsafe.Pointer(&os.Args[0]))
  argv0 := (*[1 << 30]byte)(unsafe.Pointer(argv0str.Data))[:argv0str.Len]

  n := copy(argv0, name)
  if n < len(argv0) {
    argv0[n] = 0
  }

  return nil
}

// release all the listened port or socket
// wait all clients disconnect
// send signal to let caller do cleanups and exit
func shutdown() {

  log.Println("shutting down (pid):", os.Getpid())
  // shutdown process safely
  for _,conf := range confs_ {
    conf.l.Stop()
  }
  
  _, basename := filepath.Split(execpath_)
  setProcessName("(shutting down)"+basename)
  
  wg_.Wait()

  cx_ <- syscall.SIGTERM
}

func signalHandler() {
  // this is singleton by process
  // should not be called more than once!
  c := make(chan os.Signal, 1)
  signal.Notify(c, syscall.SIGTERM, syscall.SIGHUP)
  // Block until a signal is received.
  for s := range c {
    log.Println("signal received: ", s)
    switch (s) {
    case syscall.SIGHUP:
      
      go func(){ cx_ <- s }()
      
      // restart / fork and exec
      err := reload()
      if err != nil {
        log.Println("reload err:", err)
      }
      
      return

    case syscall.SIGTERM:
      abdicate()
      shutdown()
      return
      
    }
  }
}


type Server struct {
  Network, Address    string   // eg: unix/tcp, socket/ip:port. see net.Dial
  Chmod               os.FileMode // file mode for unix socket, default 0666
  //key, cert         string // TODO: for https?
  l *stoppableListener
  Fd uintptr
}

type Context struct {
  Hash     string // suggest using config path
  User     string
  Group    string
  Maxfds   syscall.Rlimit
  Command  string
  Logfile  string
  Pidfile  string
  Directives map[string]Server
}

func validCtx(ctx Context) error {
  if (len(ctx.Hash) <= 1) {
    return errors.New("ctx.Hash is too short")
  }
 
  if (len(ctx.Directives) <= 0) {
    return errors.New("ctx.Servers is empty")
  }
  
  return nil
}

func equavalent(a Server, b Server) bool {
  return (a.Network == b.Network && a.Address == b.Address)
}

func reload() (err error) {
  // fork and exec / restart
  if _, err = os.Stat(execpath_); err != nil {
    exec, e := osext.Executable()
    if e != nil {
      return e  
    }
    if _, err = os.Stat(exec); err != nil {
      return
    }
    execpath_ = exec
  }
  
  wd, err := os.Getwd()
  if nil != err {
    return
  }

  abdicate()
    
  // write all the fds into a json string
  // from beego, code is evil but much simpler than extend net/*
  allFiles := []*os.File{}
  
  if logfile_ != nil {
    allFiles = append(allFiles, logfile_, logfile_, logfile_)
  } else {
    allFiles = append(allFiles, os.Stdin, os.Stdout, os.Stderr)
  }
  
  for k,conf := range confs_ {
    v  := reflect.ValueOf(conf.l.Listener).Elem().FieldByName("fd").Elem()
    fd := uintptr(v.FieldByName("sysfd").Int())
    f  := os.NewFile(fd, string(v.FieldByName("sysfile").String()))
    allFiles = append(allFiles, f)
   
    // presume the dupped fd by os.StartProcess is the same of the order of s.ProcAttr:Files
    conf.Fd = uintptr(len(allFiles)) - 1
    confs_[k] = conf
  }
  
  b, err := json.Marshal(confs_)
  if err != nil {
    return
  }
  inheritedinfo := string(b)
  
  os.Setenv("GOZDVAR", inheritedinfo)
  p, err := os.StartProcess(execpath_, os.Args, &os.ProcAttr{
    Dir:   wd,
    Env:   os.Environ(),
    Files: allFiles,
  })
  if nil != err {
    return 
  }
  log.Printf("child %d spawned, parent: %d\n", p.Pid, os.Getpid())
  
  // exit since process already been forked and exec 
  shutdown()
  
  return
}

func initListeners(s map[string]Server, cl chan net.Listener) error {
  // start listening 
  for k,c := range s {
    if c.Network == "unix" {
      os.Remove(c.Address)
    }
    listener, e := net.Listen(c.Network, c.Address)
    if e != nil {
      // handle error
      log.Println("bind() failed on:", c.Network, c.Address, "error:", e)
      continue
    }
    if c.Network == "unix" {
      if c.Chmod == 0 {
        c.Chmod = 0666
      }
      os.Chmod(c.Address, c.Chmod)
    }
    
    conf := s[k]
    sl := newStoppable(listener, k)
    conf.l = sl
    confs_[k] = conf
    if cl != nil {
      cl <- sl
    }
  }
  if len(confs_) <= 0{
    return errors.New("interfaces binding failed completely")
  }
  
  return nil
}

func Daemonize(ctx Context, cl chan net.Listener) (c chan os.Signal, err error) {

  err = validCtx(ctx)
  if err != nil {
    return
  }
  
  execpath_, err = exec.LookPath(os.Args[0])
  if err != nil {
    return
  }

  c       = cx_
  hash_   = ctx.Hash
  pidfile_ = ctx.Pidfile
  
  setuid(ctx.User, ctx.Group)
  
  // redirect log output, if set
  if len(ctx.Logfile) > 0 {
    logfile_, err = os.OpenFile(ctx.Logfile, os.O_WRONLY | os.O_APPEND | os.O_CREATE, os.ModeAppend | 0666)
    if err != nil {
      return
    }
    log.SetOutput(logfile_)
  }

  setrlimit(ctx.Maxfds)
  
  setuid(ctx.User, ctx.Group)

  inherited := os.Getenv("GOZDVAR");
  if len(inherited) > 0 {
    // this is the daemon
    // create a new SID for the child process
    s_ret, e := syscall.Setsid()
    if e != nil {
      err = e
      return
    }
    
    if s_ret < 0 {
      err = fmt.Errorf("Set sid failed %d", s_ret)
      return
    }
    
    // handle inherited fds
    heirs := make(map[string]Server)
    err = json.Unmarshal([]byte(inherited), &heirs)
    if err != nil {
      return 
    }

    for k,heir := range heirs {
      // compare heirs and ctx confs
      conf, ok := ctx.Directives[k];
      if !ok || !equavalent(conf, heir) {
        // do not add the listener that already been removed
        continue
      }

      f := os.NewFile(heir.Fd, k) 
      if (f == nil) {
        log.Println("unusable inherited fd", heir.Fd, "for", k)
      }
      l, e := net.FileListener(f)
      if e != nil {
        err = e
        go f.Close()
        log.Println("inherited listener binding failed", heir.Fd, "for", k, e)
        continue 
      }

      heir.l = newStoppable(l, k)
      confs_[k] = heir
      if cl != nil {
        cl <- heir.l
      }
      delete(ctx.Directives, k)
    }
    
    if (len(confs_) <= 0 && err != nil) {
      return
    }
    
    // add new listeners
    err = initListeners(ctx.Directives, cl)
    if err != nil {
      return
    }
    
    // write process info
    err = writepid()
    if err != nil {
      return 
    }

    // Handle OS signals
    // Set up channel on which to send signal notifications.
    // We must use a buffered channel or risk missing the signal
    // if we're not ready to receive when the signal is sent.
    go signalHandler()
        
    return
  }
  
  // handle reopen or stop command
  proc, err := masterproc()
  switch (ctx.Command) {
  case "stop","reopen","reload","kill":
    if err != nil {
      return
    }
    if (ctx.Command == "stop" || ctx.Command == "kill") {
      proc.Signal(syscall.SIGTERM)
    } else {
      // find old process, send SIGHUP then exit self
      // the 'real' new process running later starts by old process received SIGHUP
      proc.Signal(syscall.SIGHUP)
    }
    err = errors.New("signaling master daemon")
    return
  }
  
  // handle start(default) command
  if err == nil {
    err = errors.New("daemon already started: " + strconv.Itoa(proc.Pid))
    return
  }
  
  err = initListeners(ctx.Directives, cl)
  if err != nil {
    return
  }
  
  err = reload()
  if err != nil {
    return
  }
  
  
    
  return 
}


