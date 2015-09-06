package main
import (
    "net"
    log "github.com/Sirupsen/logrus"
    "os"
    ss "bitbucket.org/qiuyuzhou/shadowsocks/core"
    "syscall"
    "sync"
    "sync/atomic"
    "os/signal"
    "github.com/codegangsta/cli"
)

func init() {
    // Log as JSON instead of the default ASCII formatter.
    log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

    // Only log the warning severity or above.
    log.SetLevel(log.InfoLevel)
}

var config *Config

var connCount int32

func handleConnection(rawConn net.Conn) {
    var conn *ss.Conn
    var err error
    closed := false
    if conn, err = ss.NewServerConn(rawConn, &TokensManager{Config:config}); err != nil {
        return
    }
    atomic.AddInt32(&connCount, 1)
    defer func() {
        atomic.AddInt32(&connCount, -1)
        if !closed {
            conn.Close()
        }
    }()
    if err := conn.HandShake(); err != nil {
        log.Error("error handshake: ", err)
        return
    }

    host, extra, err := getRequest(conn)
    if err != nil {
        log.Error("error getting request", conn.RemoteAddr(), conn.LocalAddr(), err)
        return
    }
//    log.Debug("getting request: ", host)

    remote, err := net.Dial("tcp", host)
    if err != nil {
        if ne, ok := err.(*net.OpError); ok && (ne.Err == syscall.EMFILE || ne.Err == syscall.ENFILE) {
            // log too many open file error
            // EMFILE is process reaches open file limits, ENFILE is system limit
            log.Debug("dial error:", err)
        } else {
            log.Debug("error connecting to:", host, err)
        }
        return
    }
    defer func() {
        if !closed {
            remote.Close()
        }
    }()
    // write extra bytes read from
    if extra != nil {
        // debug.Println("getRequest read extra data, writing to remote, len", len(extra))
        if _, err = remote.Write(extra); err != nil {
            log.Error("write request extra error:", err)
            return
        }
    }

    log.WithField("addr", host).Infof("Proxy connection to %v", host)
//    log.Debugf("piping %s<->%s", conn.RemoteAddr(), host)

    go ss.PipeThenClose(conn, remote)
    ss.PipeThenClose(remote, conn)
    closed = true
//    log.Debug("closed connection to", host)
}

func waitSignal() {
    var sigChan = make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT)
    for sig := range sigChan {
        if sig == syscall.SIGHUP {
//            updatePasswd()
        } else {
            // is this going to happen?
            log.Printf("caught signal %v, exit\n", sig)
            os.Exit(0)
        }
    }
}

func run() {
    var wg sync.WaitGroup
    for _, el:= range config.Listen {
        wg.Add(1)
        go func(laddr string){
            defer wg.Done()
            ln, err := net.Listen("tcp", laddr)
            if err != nil {
                log.Fatalf("error listening at %v: %v", laddr, err)
            }
            log.WithField("listen", laddr).Infof("server listening on %v ...", laddr)
            for {
                conn, err := ln.Accept()
                if err != nil {
                    log.Errorf("accept error: %v", err)
                    continue
                }
                // Creating cipher upon first connection.
                go handleConnection(conn)
            }
        }(el)
    }
    wg.Wait()
    os.Exit(1)
}

func main() {
    app := cli.NewApp()
    app.Name = "sspserver"
    app.Usage = "Start a shadowsocks pro server."
    app.Version = "1.0pre"
    app.Author = "Charlie"

    app.Flags = []cli.Flag{
        cli.BoolFlag{
            Name: "debug,d",
            Usage: "Show debug log",
        },
        cli.StringFlag{
            Name: "config,c",
            Usage: "Run with the config file",
        },
    }

    app.Action = func(c *cli.Context) {
        if c.GlobalBool("debug") {
            log.SetLevel(log.DebugLevel)
        }

        if c.GlobalString("config") != "" {
            var err error
            config, err = ParseConfig(c.GlobalString("config"))
            if err != nil {
                log.Error(err)
                os.Exit(1)
            }
            if ok, err := config.Validate(); !ok {
                if err != nil {
                    log.Printf("Error: %v", err)
                }
                os.Exit(1)
            }
        } else {
            log.Error("Must specify config file.")
            os.Exit(1)
        }

        run()
    }
    go waitSignal()
    app.Run(os.Args)
}
