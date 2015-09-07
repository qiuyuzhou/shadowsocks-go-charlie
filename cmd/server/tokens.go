package main

import (
    "errors"
    "encoding/json"
    log "github.com/Sirupsen/logrus"
)

var (
    errNotFoundToken = errors.New("Not found the token.")
)

type TokensPlugin interface {
    Init(rawJson json.RawMessage) (error)
    GetTokenSecret(token string) (string, error)
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
            case "remote":
                plugin = &RemoteTokensPlugin{}
            default:
                log.WithField("plugin", key).Warn("Unkown tokens plugin")
                continue
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
//            log.Debug(token, s)
            return s, nil
        }
    }
    return "", errNotFoundToken
}

