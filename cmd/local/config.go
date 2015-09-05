package main

import (
    "encoding/json"
    "io/ioutil"
    "os"
    "fmt"
    ss "bitbucket.org/qiuyuzhou/shadowsocks/core"
)

type ServerEndpointConfig struct {
    Address     string      `json:"address"`
    Token       string      `json:"token"`
    TokenSecret string      `json:"token_secret"`
    Password   string      `json:"password"`
    Method     string      `json:"method"` // encryption method

    headerCipher *ss.Cipher
}

type Config struct {
    LocalAddr   string      `json:"local_addr"`

    Servers []ServerEndpointConfig `json:"servers"`
}

func ParseConfig(path string) (config *Config, err error) {
    file, err := os.Open(path) // For read access.
    if err != nil {
        return
    }
    defer file.Close()

    data, err := ioutil.ReadAll(file)
    if err != nil {
        return
    }

    config = &Config{}
    if err = json.Unmarshal(data, config); err != nil {
        return nil, err
    }

    return
}

func (c *ServerEndpointConfig)GetServerSecret() string {
    return c.Password
}

func (c *ServerEndpointConfig)GetEncryptMethod() string {
    return c.Method
}

func (c *ServerEndpointConfig)GetToken() (string, string) {
    return c.Token, c.TokenSecret
}

func (c *ServerEndpointConfig)NewHeaderCipher()*ss.Cipher {
    return c.headerCipher.Copy()
}

func (c *ServerEndpointConfig)Validate() (bool, error) {
    var err error
    valid := true
    if c.Address == "" {
        fmt.Fprintln(os.Stderr, "Must specify address for server")
        valid = false
    }
    if c.Token == "" {
        fmt.Fprintln(os.Stderr, "Must specify token for server")
        valid = false
    }
    if c.TokenSecret == "" {
        fmt.Fprintln(os.Stderr, "Must specify token_secret for server")
        valid = false
    }
    if c.Password == "" {
        fmt.Fprintln(os.Stderr, "Must specify password for server")
        valid = false
    }
    if c.Method == "" {
        fmt.Fprintln(os.Stderr, "Must specify method for server")
        valid = false
    }

    c.headerCipher, err = ss.NewCipher(c.Method, c.Password)

    return valid, err
}
