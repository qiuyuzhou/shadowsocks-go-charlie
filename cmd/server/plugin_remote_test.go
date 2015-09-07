package main
import (
    "testing"
    a "github.com/stretchr/testify/assert"
    "encoding/json"
)


func TestParseRemoteTokensPluginConfig(t *testing.T) {
    c := &RemoteTokensPluginConfig{}
    jsonData := `{"remote_server_url": "http://127.0.0.1:8000/shadowsockspro/api/tokens/%v/"}`
    json.Unmarshal([]byte(jsonData), c)
    a.Equal(t, "http://127.0.0.1:8000/shadowsockspro/api/tokens/%v/", c.RemoteServerUrl)
}
