// Use of this source code is governed by a BSD-style license

package main

import (
	"flag"
  "os"
  "fmt"
)

// configuration
var (
   optSendSignal= flag.String("s","","send signal to a master process: stop, quit, reopen, reload")
   optConfigFile= flag.String("c","","set configuration file" )
	 optHelp= flag.Bool("h",false,"this help")
)

func parseConfigFile(filePath string) bool {
	return true
}

func usage() {
  fmt.Println("[command] -conf=[config file]")
  flag.PrintDefaults()
}

func main() {
  // parse arguments
  flag.Parse()

  // -conf parse config
  if (!parseConfigFile(*optConfigFile)) {
    usage()
    os.Exit(1)
    return
  }

  // find master process id by conf

  // -s send signal to the process that has same config
  switch (*optSendSignal) {
    case "stop":  
    case "start","":
    case "reopen","reload":
  }

  // handle signals
	
}