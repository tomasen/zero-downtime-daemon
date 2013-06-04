// Use of this source code is governed by a BSD-style license

package main

import (
	"flag"
  "os"
  "fmt"
  "os/signal"
  "syscall"
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
  // Set up channel on which to send signal notifications.
  // We must use a buffered channel or risk missing the signal
  // if we're not ready to receive when the signal is sent.
  cSignal := make(chan os.Signal, 1)
  signal.Notify(cSignal, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR2, syscall.SIGINT)

  // Block until a signal is received.
  for s := range cSignal {
    fmt.Println("Got signal:", s)
    switch (s) {
      case syscall.SIGHUP, syscall.SIGUSR2:
      case syscall.SIGTERM, syscall.SIGINT:
    }
  }
  os.Exit(3)
}