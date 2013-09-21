`gozd`, is a configurable zero downtime daemon(TCP/HTTP/FCGI) framework write in golang. All it takes is integrating just one simple call to gozd.Daemonize(). Then you will get:

1. upgrade binary/service with absolutely zero downtime. high availability!

2. listen to multiple port and/or socket in same program

3. gracefully shutdown service without break and existing connections

##How to install

  go get -u bitbucket.org/PinIdea/go-zero-downtime-daemon

##Sample Code & Integration

There are sample programs in the "examples" directory.

Basic intergration steps are:

1. Initialize a channel and perpare a goroutine to handler new net.Listener 

2. Call `gozd.Daemonize(Context, chan net.Listener)` to initialize `gozd` & obtain a channel to receive exit signal from `gozd`.

3. Wait till daemon send a exit signal, do some cleanup if you want.

##Daemon Usage

> kill -TERM <pid>  send signal to gracefully shutdown daemon without break existing connections and services.

> kill -HUP <pid>  send signal to restart daemon's latest binary, without break existing connections and services.

##Daemon Configuration

<!-- language: lang-js -->
    ctx  := gozd.Context{
      Hash:[DAEMON_NAME],
      Signal:[start,stop,reload],
      Logfile:[LOG_FILEPATH,""], 
      Servers:map[string]gozd.Conf{
        [SERVER_ID]:gozd.Conf{
          Network:["unix","tcp"],
          Address:[SOCKET_FILE(eg./tmp/daemon.sock),TCP_ADDR(eg. 127.0.0.1:80)],
        },
        ...
      },
    }

##TODO

1. more examples to cover usage of:

> config file
>
> command arguments
>
> fcgi server
>
> http server

2. test cases
