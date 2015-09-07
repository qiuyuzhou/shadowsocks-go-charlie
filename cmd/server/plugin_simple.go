package main
import (
    "encoding/json"
    "errors"
    "fmt"
    log "github.com/Sirupsen/logrus"
    ss "bitbucket.org/qiuyuzhou/shadowsocks/core"
)

type SimpleTokensPlugin struct {
    Tokens map[string]string    `json:"tokens"`
}

func (self *SimpleTokensPlugin) Init(rawJson json.RawMessage) (error) {
    if err := json.Unmarshal(rawJson, &self.Tokens); err != nil {
        return err
    }

    valid := true
    for key, _:= range self.Tokens {
        if len(key) > ss.TOKEN_SIZE {
            log.WithField("token", key).Errorf("Token lenght must be less equal %v", ss.TOKEN_SIZE)
            valid = false
        }
    }
    if !valid {
        return errors.New(fmt.Sprintf("Token lenght must be less equal %v", ss.TOKEN_SIZE))
    }

    return nil
}

func (self *SimpleTokensPlugin) GetTokenSecret(token string) (string, error) {
    val, ok := self.Tokens[token]
    if !ok {
        return "", errNotFoundToken
    }
    return val, nil
}

