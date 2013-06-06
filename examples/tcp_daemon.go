// Use of this source code is governed by a BSD-style license

package main

import (
	"../"
  "net"
)

func serveTCP(*net.Conn) {
  
}

func main() {
  gozd.Daemonize()
  gozd.HandleTCPFunc(serveTCP)
  
  return
}
