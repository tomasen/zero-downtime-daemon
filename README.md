#Configurable zero downtime daemon(TCP/HTTP/FCGI) framework write in golang. 

All it takes is integrating just one simple call to gozd.Daemonize(). Then you will get:

1. upgrade binary/service with absolutely zero downtime. high availability!
2. listen to multiple port and/or socket in the same program. also able to add/remove/update them with zero downtime.
3. gracefully shutdown service without breaking any existing connections.

####Status: Acceptance testing

* * *

##How to install

    go get -u bitbucket.org/PinIdea/zero-downtime-daemon

##Sample Code & Integration

There are sample programs in the "examples" directory.

* tcp_daemon.go      
  *demonstrate typical tcp service daemon, listen to multiple socket and ports*     
* args_n_conf.go         
  *demonstrate controling daemon reload config from file by command line arguments*      
* fcgi_std.go        
  *demonstrate typical fcgi service daemon*     
* fcgi_daemon.go	  
  *demonstrate extended fcgi service daemon, able to process server params*     
* http_daemon.go	 
  *demonstrate typical http service daemon*     
* https_daemon.go	
  *demonstrate typical https service daemon*     
* mixed_daemon.go (advanced usage)
  *demonstrate mixed service(tcp/fcgi/http/https) daemon, listen to diffrent socket and ports*

Basic integration steps are:

1. Initialize a channel and prepare a goroutine to handler new net.Listener 
2. Call `gozd.Daemonize(Context, chan net.Listener)` to initialize `gozd` & obtain a channel to receive exit signal from `gozd`.
3. Wait till daemon send a exit signal, do some cleanup if you want.

##Daemon Usage

> kill -TERM <pid>  send signal to gracefully shutdown daemon without breaking existing connections and services.

> kill -HUP <pid>  send signal to start daemon's latest binary, without breaking existing connections and services, and also absolutely zero downtime. old process will be gracefully shut down.

##Daemon Configuration

    ctx  := gozd.Context{
      Hash:[DAEMON_NAME],
      Command:[start,stop,reload],
      Logfile:[LOG_FILEPATH,""], 
      Maxfds: {[RLIMIT_NOFILE_SOFTLIMIT],[RLIMIT_NOFILE_HARDLIMIT]}
      User:   [USERID],
      Group:  [GROUPID],
      Directives:map[string]gozd.Server{
        [SERVER_ID]:gozd.Server{
          Network:["unix","tcp"],
          Address:[SOCKET_FILE(eg./tmp/daemon.sock),TCP_ADDR(eg. 127.0.0.1:80)],
          Chmod:0666,
        },
        ...
      },
    }
    cl := make(chan net.Listener,1)
    go handleListners(cl)
    sig, err := gozd.Daemonize(ctx, cl) 
    // ...
    for s := range sig  {
      switch s {
      case syscall.SIGHUP, syscall.SIGUSR2:
        // do some custom jobs while reload/hotupdate
      
    
      case syscall.SIGTERM:
        // do some clean up and exit
        return
      }
    }
   

##Functions

###func Daemonize
    func Daemonize(ctx Context, cl chan net.Listener) (c chan bool, err error)
    
###type Context
    type Context struct {
        Hash       string
        User       string
        Group      string
        Maxfds     syscall.Rlimit
        Command    string
        Logfile    string
        Pidfile    string
        Directives map[string]Server
    }

###type Server
    type Server struct {
        Network, Address string      // eg: unix/tcp, socket/ip:port. see net.Dial
        Chmod            os.FileMode // file mode for unix socket, default 0666
    }

##Typical usage

Gateway, Load Balancer, Stateless Service
  
##TODO

test cases

  + race condition test
  + stress test
  + more context config validations
  + better default place to store and lock pid

##How to contribute

Help is needed to write more test cases and stress test.

Patches or suggestions that can make the code simpler or easier to use are welcome to submit to [issue area](https://bitbucket.org/PinIdea/go-zero-downtime-daemon/issues?status=new&status=open).

##How it works

The basic principle: master process fork the process, and child process evecve corresponding binary which inherit the file descriptions of the listening port and/or socket. 

`os.StartProcess` did the trick to append files that contains handle that is can be inherited. Then the child process can start listening from same handle which we passed fd number via environment variable as index. After that we use `net.FileListener` to recreate net.Listener interface to gain access to the socket created by last master process.

We also expand the net.Listener and net.Conn, so that the master process will stop accept new connection and wait until all existing connection to dead naturally before exit the process. 

The detail in in the code of reload() in daemon.go. 

##Special Thanks

The zero downtime idea and code is inspired by nginx and beego. Thanks.

