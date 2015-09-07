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

type RemoteTokensPluginQueryRequest struct {
    Token string
    resultChan chan string
}

type RemoteTokensPluginConfig struct {
    RemoteServerUrl string      `json:"remote_server_url"`
}

type RemoteTokensPlugin struct {
    tokensCache *cache.Cache
    config RemoteTokensPluginConfig

    requestsChan chan *RemoteTokensPluginQueryRequest
    responsesChan chan *RemoteTokensPluginToken

    pendingRequests map[string][]chan string
}

func (self *RemoteTokensPlugin) Init(rawJson json.RawMessage) (error) {
    if err := json.Unmarshal(rawJson, &self.config); err != nil {
        return err
    }

    self.tokensCache = cache.New(5*time.Minute, 30*time.Second)

    self.pendingRequests = make(map[string][]chan string)

    self.requestsChan = make(chan *RemoteTokensPluginQueryRequest, 32)
    self.responsesChan = make(chan *RemoteTokensPluginToken, 32)
    go self.svc()

    return nil
}

func (self *RemoteTokensPlugin) svc() {
    log.Info("Start RemoteTokensPlugin svc...")
    for {
        select {
        case request := <-self.requestsChan:
            val, ok := self.tokensCache.Get(request.Token)
            if ok {
                request.resultChan <- val.(string)
                continue
            }

            resultChans := self.pendingRequests[request.Token]
            self.pendingRequests[request.Token] = append(resultChans, request.resultChan)

            go func(token string) {
                log.WithField("token", token).Debug("Start query token from remote server.")
                t := RemoteTokensPluginToken{}

                param := &napping.Params{"format": "json"}
                resp, err := napping.Get(fmt.Sprintf(self.config.RemoteServerUrl, token), param, &t, nil)
                if err == nil {
                    if resp.Status() == 200 {
                        log.WithFields(log.Fields{
                            "token": t.Token,
                            "token_serect": t.TokenSecret,
                        }).Debugf("Get token secret from remote")
                        self.responsesChan <- &t
                        return
                    } else if resp.Status() != 404 {
                        log.WithField("status", resp.Status()).Error("Query token from remote failed.")
                    }
                } else {
                    log.WithField("error", err).Error("Query token from remote failed.")
                }
                t.Token = token
                self.responsesChan <- &t
            }(request.Token)
        case response := <-self.responsesChan:
            if response.TokenSecret != "" {
                self.tokensCache.Set(response.Token, response.TokenSecret, 0)
            }
            resultChans := self.pendingRequests[response.Token]
            for _, c := range resultChans {
                c <- response.TokenSecret
            }
            delete(self.pendingRequests, response.Token)
        }
    }
}

func (self *RemoteTokensPlugin) GetTokenSecret(token string) (string, error) {
    val, ok := self.tokensCache.Get(token)
    if ok {
        return val.(string), nil
    }

    request := &RemoteTokensPluginQueryRequest{Token:token}
    self.requestsChan <- request
    tokenSecret := <- request.resultChan
    if tokenSecret == "" {
        return tokenSecret, errNotFoundToken
    }
    return tokenSecret, nil
}
