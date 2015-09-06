package main
import (
    "testing"
    a "github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
    config, err := ParseConfig("testdata/config.json")
    if err != nil {
        t.Fatal("error parsing config.json:", err)
    }
//    if config.LocalPort != 1080 {
//        t.Error("wrong local port from config")
//    }
    if len(config.Servers) != 2 {
        t.Error("wrong servers array length from config")
    }
    ep := config.Servers[0]
    a.Equal(t, ep.Address, "192.168.1.1:8388", "wrong host @ ep0")
    a.Equal(t, ep.Method, "aes-256-cfb", "wrong method @ ep0")
}

func TestParseUrl(t *testing.T) {
    ep, err := parseSSPUrl("ssp://aes-256-cfb:shared_secret@127.0.0.1:8388/charlie/0123456789abcdefg/")
    a.Nil(t, err)
    a.Equal(t, ep.Method, "aes-256-cfb", "wrong method")
    a.Equal(t, ep.Token, "charlie", "wrong token")
    a.Equal(t, ep.TokenSecret, "0123456789abcdefg", "wrong token secret")
}