package main
import (
    "net"
    "io"
    "fmt"
    ss "bitbucket.org/qiuyuzhou/shadowsocks/core"
    "encoding/binary"
    "strconv"
)

func getRequest(conn *ss.Conn) (host string, extra []byte, err error) {
    const (
        idType  = 0 // address type index
        idIP0   = 1 // ip addres start index
        idDmLen = 1 // domain address length index
        idDm0   = 2 // domain address start index

        typeIPv4 = 1 // type is ipv4 address
        typeDm   = 3 // type is domain address
        typeIPv6 = 4 // type is ipv6 address

        lenIPv4   = 1 + net.IPv4len + 2 // 1addrType + ipv4 + 2port
        lenIPv6   = 1 + net.IPv6len + 2 // 1addrType + ipv6 + 2port
        lenDmBase = 1 + 1 + 2           // 1addrType + 1addrLen + 2port, plus addrLen
    )

    // buf size should at least have the same size with the largest possible
    // request size (when addrType is 3, domain name has at most 256 bytes)
    // 1(addrType) + 1(lenByte) + 256(max length address) + 2(port)
    buf := make([]byte, 260)
    var n int
    // read till we get possible domain length field
//    ss.SetReadTimeout(conn)
    if n, err = io.ReadAtLeast(conn, buf, idDmLen+1); err != nil {
        return
    }

    reqLen := -1
    switch buf[idType] {
        case typeIPv4:
        reqLen = lenIPv4
        case typeIPv6:
        reqLen = lenIPv6
        case typeDm:
        reqLen = int(buf[idDmLen]) + lenDmBase
        default:
        err = fmt.Errorf("addr type %d not supported", buf[idType])
        return
    }

    if n < reqLen { // rare case
        if _, err = io.ReadFull(conn, buf[n:reqLen]); err != nil {
            return
        }
    } else if n > reqLen {
        // it's possible to read more than just the request head
        extra = buf[reqLen:n]
    }

    // Return string for typeIP is not most efficient, but browsers (Chrome,
    // Safari, Firefox) all seems using typeDm exclusively. So this is not a
    // big problem.
    switch buf[idType] {
        case typeIPv4:
        host = net.IP(buf[idIP0 : idIP0+net.IPv4len]).String()
        case typeIPv6:
        host = net.IP(buf[idIP0 : idIP0+net.IPv6len]).String()
        case typeDm:
        host = string(buf[idDm0 : idDm0+buf[idDmLen]])
    }
    // parse port
    port := binary.BigEndian.Uint16(buf[reqLen-2 : reqLen])
    host = net.JoinHostPort(host, strconv.Itoa(int(port)))
    return
}
