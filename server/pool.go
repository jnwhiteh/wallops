package main

import (
	"fmt"
	"sync"
)

var (
	generateTokenError = fmt.Errorf("Failed to generate token")
	invalidTokenError  = fmt.Errorf("Invalid token")
)

type connectionPooler interface {
	Connect(config ServerConfig) (string, error)
	Unregister(token string) error
}

type pool struct {
	// A map from server configuration to connection
	conns map[ServerConfig]*Proxy

	// A map from token to connection
	tokenMap map[string]*Proxy

	sync.RWMutex
}

func NewConnectionPool() connectionPooler {
	return &pool{
		conns:    make(map[ServerConfig]*Proxy),
		tokenMap: make(map[string]*Proxy),
	}
}

func (p *pool) Unregister(token string) error {
	p.Lock()
	delete(p.tokenMap, token)
	p.Unlock()

	return nil
}

// Connect will connect to a server based on configuration or re-use an
// existing open connection. If successful, a token that can be used to
// communicate with the connection is returned.
func (p *pool) Connect(config ServerConfig) (string, error) {
	p.Lock()
	conn, ok := p.conns[config]
	p.Unlock()

	if ok && conn != nil {
		token, err := generateToken()
		p.tokenMap[token] = conn
		return token, err
	}

	// Create a new connection
	conn, err := NewConnection(config)
	if err != nil {
		return "", err
	}

	token, err := generateToken()
	if err != nil {
		return "", generateTokenError
	}

	p.Lock()
	p.tokenMap[token] = conn
	p.Unlock()

	return token, nil
}
