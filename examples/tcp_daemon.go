// Use of this source code is governed by a BSD-style license

package main

import (
	"../"

)

func handleTCPConn(*net.Conn) {
  
}

func main() {
  gozd.Daemonize()
  gozd.HandleTCPConnection(handleTCPConn)
  
  return 0
}
