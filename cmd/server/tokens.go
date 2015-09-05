package main

import (
    "errors"
)

var (
    errNotFoundToken = errors.New("Not found the token.")
)

type TokensManager struct {
    *Config
}

func (self *TokensManager) GetTokenSecret(token string)(string, error) {
    if self.Tokens == nil {
        return "", errNotFoundToken
    }

    val, ok := self.Config.Tokens[token]
    if !ok {
        return "", errNotFoundToken
    }
    return val, nil
}
