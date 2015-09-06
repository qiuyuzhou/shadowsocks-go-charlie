package main
import (
    "net"
    log "github.com/Sirupsen/logrus"
    ss "bitbucket.org/qiuyuzhou/shadowsocks/core"
    "github.com/codegangsta/cli"
    "os"
)

func init() {
    // Log as JSON instead of the default ASCII formatter.
    log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

    // Only log the warning severity or above.
    log.SetLevel(log.InfoLevel)
}

var config = &Config{}

func createServerConn(rawaddr []byte, addr string) (remote *ss.Conn, err error) {
    ep := config.Servers[0]
    remote, err = ss.DialWithRawAddr(rawaddr, ep.Address, ep)
    return
}

func handleConnection(conn net.Conn) {
    closed := false
    defer func() {
        if !closed {
            conn.Close()
        }
    }()

    var err error = nil
    if err = handShake(conn); err != nil {
        log.Warning("socks handshake:", err)
        return
    }
//    log.Debug("socks5 connection handshaked!")
    rawaddr, addr, err := getRequest(conn)
    if err != nil {
        log.Warning("error getting request:", err)
        return
    }
//    log.Debugf("socks5 connection get request: %v", addr)

    remote, err := createServerConn(rawaddr, addr)
    if err != nil {
        log.Debugf("error when create connection to server: %v\n", err)
        return
    }
    defer func() {
        if !closed {
            remote.Close()
        }
    }()

    // Sending connection established message immediately to client.
    // This some round trip time for creating socks connection with the client.
    // But if connection failed, the client will get connection reset error.
    _, err = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x08, 0x43})
    if err != nil {
        log.WithField("error", err).Debug("send connection confirmation error")
        return
    }

    log.WithField("addr", addr).Infof("Proxy connection to %v", addr)

//    log.Debugf("piping %s<->%s", conn.RemoteAddr(), remote.RemoteAddr())

    go ss.PipeThenClose(conn, remote)
    ss.PipeThenClose(remote, conn)
    closed = true
//    log.Debug("closed connection to", addr)
}

func run(listenAddr string) {
    ln, err := net.Listen("tcp", listenAddr)
    if err != nil {
        log.Fatal(err)
    }
    log.WithField("listen", listenAddr).Infof("starting local socks5 server listen on %v ...", listenAddr)
    for {
        conn, err := ln.Accept()
        if err != nil {
            log.WithField("error", err).Warning("Accept connection error.")
            continue
        }
        go handleConnection(conn)
    }
}

func main() {
    app := cli.NewApp()
    app.Name = "ssplocal"
    app.Usage = "Start a socks5 proxy which forward connections to a shadowsocks pro server."
    app.Version = "1.0pre"
    app.Author = "Charlie"

    app.Flags = []cli.Flag{
        cli.StringFlag{
            Name:  "listen,l",
            Value: "127.0.0.1:1080",
            Usage: "Local Socks5 proxy server listen address",
        },
        cli.IntFlag{
            Name:  "timeout,t",
            Value: 350,
            Usage: "Network timeout in minisecond",
        },
        cli.StringSliceFlag{
            Name: "server,s",
            Usage: "specify ssp server with url format. \n\tExample: ssp://method:password@host/token/token_secret/",
        },
        cli.BoolFlag{
            Name: "debug,d",
            Usage: "Show debug log",
        },
    }

    app.Action = func(c *cli.Context) {
        if c.GlobalBool("debug") {
            log.SetLevel(log.DebugLevel)
        }

        config.LocalAddr = c.GlobalString("listen")
        servers := c.GlobalStringSlice("server")
        if len(servers) == 0 {
            log.Error("Give at least one server url by flag --server")
            os.Exit(1)
        }

        serverEpConfigs := make([]*ServerEndpointConfig, 0, 5)
        {
            hasError := false
            for _, v := range servers {
                sc, err := parseSSPUrl(v)
                if err != nil {
                    log.Error(err)
                    hasError = true
                } else {
                    serverEpConfigs = append(serverEpConfigs, sc)
                }
            }
            if hasError {
                os.Exit(1)
            }
        }
        config.Servers = serverEpConfigs

        run(config.LocalAddr)
    }
    app.Run(os.Args)
}

