package main

import (
    "errors"
    "encoding/json"
    log "github.com/Sirupsen/logrus"
    ss "bitbucket.org/qiuyuzhou/shadowsocks/core"
    "fmt"
)

var (
    errNotFoundToken = errors.New("Not found the token.")
)

type TokensPlugin interface {
    Init(rawJson json.RawMessage) (error)
    GetTokenSecret(token string) (string, error)
}

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
    if self.Tokens == nil {
        return "", errNotFoundToken
    }

    val, ok := self.Tokens[token]
    if !ok {
        return "", errNotFoundToken
    }
    return val, nil
}

type TokensManager struct {
    *Config
    plugins []TokensPlugin
}

func NewTokensManager(config *Config) (*TokensManager, error) {
    m := &TokensManager{Config: config, plugins: make([]TokensPlugin, 0, 8)}

    for key, value := range m.Config.TokensPlugins {
        var plugin TokensPlugin
        switch key {
            case "simple":
                plugin = &SimpleTokensPlugin{}
            default:
                log.WithField("plugin", key).Warn("Unkown tokens plugin")
        }
        if err := plugin.Init(value); err != nil {
            return nil, err
        }
        log.WithField("plugin", key).Info("Tokens Plugin initialed.")
        m.plugins = append(m.plugins, plugin)
    }

    return m, nil
}

func (self *TokensManager) GetTokenSecret(token string) (string, error) {
    for _, plugin := range self.plugins {
        s, err := plugin.GetTokenSecret(token)
        if err == nil {
            return s, nil
        }
    }
    return "", errNotFoundToken
}

