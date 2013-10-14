package gozd

import (
  "net"
  "errors"
  "sync/atomic"
)  

var (
  ErrorAlreadyStopped = errors.New("listener already been stoped")
)

// Allows for us to notice when the connection is closed.
type conn struct {
  net.Conn
  isclose int32
}

func (c conn) Close() error {
  err := c.Conn.Close()
  if atomic.CompareAndSwapInt32(&c.isclose, 0, 1) && err == nil {
    wg_.Done()
  }
  return err
}

type stoppableListener struct {
  net.Listener
  Name    string
  stopped int32 
}

func newStoppable(l net.Listener, n string) (sl *stoppableListener) {
  sl = &stoppableListener{Listener: l, Name: n, stopped:0}
  return
}

func (sl *stoppableListener) Accept() (c net.Conn, err error) {
  if atomic.LoadInt32(&sl.stopped) == 1 {
    return nil, ErrorAlreadyStopped
  }
  
  c, err = sl.Listener.Accept()
  if err != nil {
    return
  }
  wg_.Add(1)
  // Wrap the returned connection, so that we can observe when
  // it is closed.
  c = conn{Conn: c, isclose: 0}
  return
}

func (sl *stoppableListener) Stop() {
  atomic.StoreInt32(&sl.stopped, 1)
  // do not close because net.UnixListener will unlink 
  // socket file on close and mess things up
  // sl.Listener.Close()
}
