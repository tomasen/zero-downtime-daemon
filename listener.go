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
 */

package gozd

import (
  "net"
  "os"
)

// Override Close() method in net.Conn interface
func (c *Conn) Close() error {
  Log("GOZDConn Closed.")
  openedGOZDConns.Remove(c.element)
  return c.Conn.Close() // call net.Conn.Close()
}

func newGOZDListener(netType, laddr, groupName string) (*gozdListener, error) {
  var l net.Listener
  var err error

  // find if already exists by groupname
  if openedFDs[groupName] != nil {
    Log("Listen with opened FDs: [%d][%s][%s]", openedFDs[groupName].fd, openedFDs[groupName].name, groupName)
    f := os.NewFile(uintptr(openedFDs[groupName].fd), openedFDs[groupName].name)
    l, err = net.FileListener(f)
  } else {
    l, err = net.Listen(netType, laddr)
  }

  l_gozd := new(gozdListener)
  l_gozd.Listener = l
  return l_gozd, err
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

func stopListening() {
  for _, v := range registeredGOZDHandler {
    l := v.listener.Listener
    err := l.Close()
    if err != nil {
      LogErr(err.Error())
    }
  }
}

