package core

import (
    "encoding/binary"
    "fmt"
    "io"
    "net"
    "strconv"
    "errors"
    "bytes"
)

const TOKEN_SIZE = 16

type PasswordQueryFn func(tokenId string)string

type BaseEncryptConfig interface {
    GetServerSecret()string
    GetEncryptMethod()string
    NewHeaderCipher()*Cipher
}

type ClientEncryptConfig interface {
    BaseEncryptConfig
    GetToken()(string, string)
}

type ServerEncryptConfig interface {
    BaseEncryptConfig
    GetTokenSecret(token string)(string, error)
}

type Conn struct {
    net.Conn
    headerCipher  *Cipher
    bodyCipher    *Cipher
    serverEncryptConfig ServerEncryptConfig
    clientEncryptConfig ClientEncryptConfig
    readBuf  []byte
    writeBuf []byte
}

func NewClientConn(c net.Conn, encryptConfig ClientEncryptConfig) (*Conn, error) {
    return &Conn{
        Conn:     c,
        headerCipher: encryptConfig.NewHeaderCipher(),
        clientEncryptConfig: encryptConfig,
        readBuf:  leakyBuf.Get(),
        writeBuf: leakyBuf.Get()}, nil
}

func NewServerConn(c net.Conn, encryptConfig ServerEncryptConfig) (*Conn, error) {
    cipher, err:= NewCipher(encryptConfig.GetEncryptMethod(), encryptConfig.GetServerSecret())
    if err != nil {
        return nil, err
    }
    return &Conn{
        Conn:     c,
        headerCipher: cipher,
        serverEncryptConfig: encryptConfig,
        readBuf:  leakyBuf.Get(),
        writeBuf: leakyBuf.Get()}, nil
}

func RawAddr(addr string) (buf []byte, err error) {
    host, portStr, err := net.SplitHostPort(addr)
    if err != nil {
        return nil, fmt.Errorf("shadowsocks: address error %s %v", addr, err)
    }
    port, err := strconv.Atoi(portStr)
    if err != nil {
        return nil, fmt.Errorf("shadowsocks: invalid port %s", addr)
    }

    hostLen := len(host)
    l := 1 + 1 + hostLen + 2 // addrType + lenByte + address + port
    buf = make([]byte, l)
    buf[0] = 3             // 3 means the address is domain name
    buf[1] = byte(hostLen) // host address length  followed by host address
    copy(buf[2:], host)
    binary.BigEndian.PutUint16(buf[2+hostLen:2+hostLen+2], uint16(port))
    return
}

// This is intended for use by users implementing a local socks proxy.
// rawaddr shoud contain part of the data in socks request, starting from the
// ATYP field. (Refer to rfc1928 for more information.)
func DialWithRawAddr(rawaddr []byte, server string, encryptConfig ClientEncryptConfig) (c *Conn, err error) {
    var conn net.Conn
    if conn, err = net.Dial("tcp", server); err != nil {
        return
    }
    if c, err = NewClientConn(conn, encryptConfig); err != nil {
        return
    }
    if err = c.HandShake(); err != nil {
        return
    }
    if _, err = c.Write(rawaddr); err != nil {
        c.Close()
        return nil, err
    }
    return
}

// addr should be in the form of host:port
func Dial(addr, server string, encryptConfig ClientEncryptConfig) (c *Conn, err error) {
    ra, err := RawAddr(addr)
    if err != nil {
        return
    }
    return DialWithRawAddr(ra, server, encryptConfig)
}

func (c *Conn) Close() error {
    leakyBuf.Put(c.readBuf)
    leakyBuf.Put(c.writeBuf)
    return c.Conn.Close()
}

func (c *Conn) initBodyCipher(method, tokenSecret string)(err error) {
    c.bodyCipher, err = NewCipher(method, tokenSecret)
    if err != nil {
        return
    }

    var iv []byte
    if iv, err = c.bodyCipher.initEncrypt(); err != nil {
        return
    }
    c.Conn.Write(iv)

    iv = make([]byte, c.bodyCipher.info.ivLen)
    if _, err = io.ReadFull(c.Conn, iv); err != nil {
        return
    }
    if err = c.bodyCipher.initDecrypt(iv); err != nil {
        return
    }
    return
}

func (c *Conn) HandShake() (err error) {
    if c.serverEncryptConfig != nil {
        {
            iv := make([]byte, c.headerCipher.info.ivLen)
            if _, err = io.ReadFull(c.Conn, iv); err != nil {
                return
            }
            if err = c.headerCipher.initDecrypt(iv); err != nil {
                return
            }
        }

        buf := c.readBuf[:TOKEN_SIZE]
        if _, err = io.ReadFull(c.Conn, buf); err != nil {
            return
        }

        decryptBuf := c.readBuf[TOKEN_SIZE:TOKEN_SIZE+TOKEN_SIZE]
        c.headerCipher.decrypt(decryptBuf, buf)
        var token string
        {
            i := bytes.IndexByte(decryptBuf, 0)
            if i != -1 {
                token = string(decryptBuf[:i])
            } else {
                token = string(decryptBuf)
            }
        }

        c.headerCipher = nil

        var tokenSecret string
        tokenSecret, err = c.serverEncryptConfig.GetTokenSecret(token)
        if err != nil {
            return
        }

        err = c.initBodyCipher(c.serverEncryptConfig.GetEncryptMethod(), tokenSecret)

        return
    } else if c.clientEncryptConfig != nil {
        token, tokenSecret := c.clientEncryptConfig.GetToken()
        if len(token) > TOKEN_SIZE {
            return errors.New("Wrong token length")
        }
        var iv []byte
        iv, err = c.headerCipher.initEncrypt()
        if err != nil {
            return
        }
        cipherData := c.writeBuf[:len(iv)+TOKEN_SIZE]
        if iv != nil {
            copy(cipherData, iv)
        }
        var tokenBytes = make([]byte, TOKEN_SIZE)
        copy(tokenBytes, []byte(token))
        // Padding the token to TOKEN_SIZE
        if len(token) < TOKEN_SIZE {
            paddingLen := TOKEN_SIZE - len(token)
            copy(tokenBytes[len(token):], bytes.Repeat([]byte{byte(0)}, paddingLen))
        }

        c.headerCipher.encrypt(cipherData[len(iv):], tokenBytes)
        _, err = c.Conn.Write(cipherData)
        if err != nil {
            return
        }
        c.headerCipher = nil
        err = c.initBodyCipher(c.clientEncryptConfig.GetEncryptMethod(), tokenSecret)

        return
    }
    panic("No client encrypt config")
}

func (c *Conn) Read(b []byte) (n int, err error) {
    cipherData := c.readBuf
    if len(b) > len(cipherData) {
        cipherData = make([]byte, len(b))
    } else {
        cipherData = cipherData[:len(b)]
    }

    n, err = c.Conn.Read(cipherData)
    if n > 0 {
        c.bodyCipher.decrypt(b[0:n], cipherData[0:n])
    }
    return
}

func (c *Conn) Write(b []byte) (n int, err error) {
    cipherData := c.writeBuf
    dataSize := len(b)
    if dataSize > len(cipherData) {
        cipherData = make([]byte, dataSize)
    } else {
        cipherData = cipherData[:dataSize]
    }

    c.bodyCipher.encrypt(cipherData, b)
    n, err = c.Conn.Write(cipherData)
    return
}
