`gozd`, is a configurable zero downtime daemon(TCP/HTTP/FCGI) framework write in golang.

##How to install

    go get -u bitbucket.org/kardianos/osext
    go get -u bitbucket.org/PinIdea/go-zero-downtime-daemon

##Sample Code & Integration

There are sample `TCP/HTTP/FCGI` programs in examples:

    package main
    
    import (
      "bitbucket.org/PinIdea/go-zero-downtime-daemon"
    )
    
    func serveTCP(conn gozd.Conn) {
      // your own TCP handler
    }
    
    func main() {
      daemonChan := gozd.Daemonize()
      gozd.RegistHandler("Group0", "serveTCP", serveTCP)
      if err != nil {
        fmt.Println(err.Error())
        return
      }
    
      // wait till daemon send a exit signal
      <-daemonChan
    }

These are the major intergration steps:

1. Call `Daemonize()` to initialize `gozd` & obtain a channel to receive exit signal from `gozd`.
2. Call `gozd.RegistHandler()` to regist your own handler function, its parameters MUST contain a `gozd.Conn` type, which encapsulates `net.Conn` you used before. Replace `net.Conn` with `gozd.Conn`. However, you HAVE to use configuration file to tell gozd ports your program listening instead of `Listen()` to these ports yourself.
3. Run your own logic, main goroutine MUST not be blocked in your own logic.
4. Wait till daemon send a exit signal, do some cleanup if you want.

##Daemon Usage

Once you build your program based on gozd, you can use following command line arguments to start the daemon and other operations.  A daemon configuration file MUST be prepared for your program.

    -s Send signal to old process: <start, stop, quit, reopen, reload>.

    -c Set configuration file path.

    -f Running foreground for debug, recommended if you are using GDB or other debuggers.

    -v Show GOZD log.

> kill -HUP <pid>  send signal to restart daemon's latest binary, without break current connections and services.

##Daemon Configuration

    [Group0]
    mode     [tcp|http|https|fcgi]
    listen   [ip|port|unix socket]
    
    [Group1]
    mode     [tcp|http|https|fcgi]
    listen   [ip|port|unix socket]
    
    [Group2]
    mode     [tcp|http|https|fcgi]
    listen   [ip|port|unix socket]
    key      <path of ssl key file>
    cert     <path of ssl cert file>

##How to test

Once you have done intergration, do these steps to test if daemon is working properly:

1. Use -v & -f to run, make sure no [GOZDERR] message displayed. Without -f, all output of your app will be redirected to `/dev/null` by default.

2. Connect to your app, the client should take a keep-alive connection with port or socket defined in config file.

3. Use `kill -HUP <your app's pid>` while the client should not be disconnected.

4. Start another client, do step 2 again.

5. Use `ps aux | grep <your app name>` or any other tool you like, 2 process of your app should running.

6. If at least one of your app process is already running, using `-s start` to start another instance should NOT work.
