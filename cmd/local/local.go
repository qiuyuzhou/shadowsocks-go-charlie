package main
import (
    "net"
    log "github.com/Sirupsen/logrus"
    ss "bitbucket.org/qiuyuzhou/shadowsocks/core"
    "time"
    "flag"
    "os"
    "fmt"
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
    remote, err = ss.DialWithRawAddr(rawaddr, ep.Address, &ep)
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
    var configFile string
    var cmdConfig = &Config{
        Servers:make([]ServerEndpointConfig, 1)}
    var printVer, help, debug bool
    var err error

    flag.BoolVar(&printVer, "version", false, "print version")
    flag.StringVar(&configFile, "c", "config.json", "specify config file")
    flag.StringVar(&cmdConfig.LocalAddr, "l", "127.0.0.1:1080", "local socks5 server address")
    flag.StringVar(&cmdConfig.Servers[0].Address, "s", "", "server address")
    flag.StringVar(&cmdConfig.Servers[0].Method, "m", "", "encryption method, default: aes-256-cfb")
    flag.StringVar(&cmdConfig.Servers[0].Password, "p", "", "password")
    flag.StringVar(&cmdConfig.Servers[0].Token, "t", "", "Token")
    flag.StringVar(&cmdConfig.Servers[0].TokenSecret, "k", "", "Token secret")
    flag.BoolVar(&help, "h", false, "print usage")
    flag.BoolVar(&debug, "debug", false, "print debug message")

    flag.Parse()
    if help {
        ss.PrintVersion()
        fmt.Println("Usage:")
        flag.PrintDefaults()
        os.Exit(0)
    }
    if printVer {
        ss.PrintVersion()
        os.Exit(0)
    }
    if debug {
        log.SetLevel(log.DebugLevel)
    }

    exists, _ := ss.IsFileExists(configFile)

    if exists {
        config, err = ParseConfig(configFile)
        if err != nil {
            if !os.IsNotExist(err) {
                fmt.Fprintf(os.Stderr, "error reading %s: %v\n", configFile, err)
                os.Exit(1)
            }
        }
        log.WithField("File", configFile).Infof("use config file: %v.", configFile)
        log.Debugf("%v", config)
    } else {
        config = cmdConfig
    }

    if config.Servers[0].Method == "" {
        config.Servers[0].Method = "aes-256-cfb"
    }

    for i := 0; i < len(config.Servers); i++ {
        if ok, err := config.Servers[i].Validate(); !ok {
            if err != nil {
                log.Error(err)
            }
            os.Exit(1)
        }
    }

    run(config.LocalAddr)

}

