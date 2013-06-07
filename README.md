gozd, is a configurable zero downtime daemon(TCP/HTTP/FCGI) framework write in golang.

##Sample Code

There are sample TCP/HTTP/FCGI programs in examples
```
  package main

  import (
    "bitbucket.org/PinIdea/go-zero-downtime-daemon"
  )

  func main() {
    gozd.Daemonize()
    return
  }
```

##Daemon Usage

Once you build your program based on gozd, you can use following command line to start the daemon and other operations.

  -s send signal to a master process: stop, quit, reopen, reload
  -c set configuration file
  
kill -HUP <pid>  send signal to restart daemon's latest binary without break current connections and services.

##Daemon Configuration
```
  [Group0]
  mode     [tcp|http|fcgi]
  listen   [ip|port|unix socket]

  [Group1]
  mode     [tcp|http|fcgi]
  listen   [ip|port|unix socket]
```