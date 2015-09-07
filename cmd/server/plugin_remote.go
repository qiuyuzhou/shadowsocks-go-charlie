package main
import (
    cache "github.com/pmylund/go-cache"
    "encoding/json"
    "time"
    napping "gopkg.in/jmcvetta/napping.v1"
    "fmt"
    log "github.com/Sirupsen/logrus"
)

type RemoteTokensPluginToken struct {
    Token   string      `json:"token"`
    TokenSecret string  `json:"token_secret"`
}

type RemoteTokensPluginConfig struct {
    RemoteServerUrl string      `json:"remote_server_url"`
}

type RemoteTokensPlugin struct {
    tokensCache *cache.Cache
    config RemoteTokensPluginConfig
}

func (self *RemoteTokensPlugin) Init(rawJson json.RawMessage) (error) {
    if err := json.Unmarshal(rawJson, &self.config); err != nil {
        return err
    }

    self.tokensCache = cache.New(5*time.Minute, 30*time.Second)

    return nil
}

//func (self *RemoteTokensPlugin) svc() {
//}

func (self *RemoteTokensPlugin) GetTokenSecret(token string) (string, error) {
    val, ok := self.tokensCache.Get(token)
    if ok {
        return val.(string), nil
    }

    t := RemoteTokensPluginToken{}

    param := &napping.Params{"format": "json"}
    resp, err := napping.Get(fmt.Sprintf(self.config.RemoteServerUrl, token), param, &t, nil)
    if err != nil {
        return "", err
    }
    if resp.Status() == 200 {
        log.Debug(t)
        self.tokensCache.Add(t.Token, t.TokenSecret, 0)
        return t.TokenSecret, nil
    }

    return "", errNotFoundToken
}
