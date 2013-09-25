package gozd

import (
  "net"
  "sync"
  "errors"
)  

var (
  ErrorAlreadyStopped = errors.New("listener already been stoped")
)

// Allows for us to notice when the connection is closed.
type conn struct {
  net.Conn
  wg      *sync.WaitGroup
  isclose bool
  lock    sync.Mutex
}

func (c conn) Close() error {
  c.lock.Lock()
  defer c.lock.Unlock()
  err := c.Conn.Close()
  if !c.isclose && err == nil {
    c.wg.Done()
    c.isclose = true
  }
  return err
}

type stoppableListener struct {
  net.Listener
  Name    string
  stopped bool
  wg      *sync.WaitGroup
}

func newStoppable(l net.Listener, w *sync.WaitGroup, n string) (sl *stoppableListener) {
  sl = &stoppableListener{Listener: l, wg: w, Name: n}
  return
}

func (sl *stoppableListener) Accept() (c net.Conn, err error) {
  if sl.stopped == true {
    return nil, ErrorAlreadyStopped
  }
  c, err = sl.Listener.Accept()
  if err != nil {
    return
  }
  sl.wg.Add(1)
  // Wrap the returned connection, so that we can observe when
  // it is closed.
  c = conn{Conn: c, wg: sl.wg}
  return
}

func (sl *stoppableListener) Stop() {
  sl.stopped = true
  // do not close because net.UnixListener will unlink 
  // socket file on close and mess things up
  // sl.Listener.Close()
}
