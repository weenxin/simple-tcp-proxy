package proxy

import (
	"errors"
	"fmt"
	"github.com/weenxin/simple-tcp-proxy/server"
	"sync"
)

var (
	ErrClientCountExceeded             = errors.New("client count exceeded")
	ErrBadRequest                      = errors.New("bad request, query should begin with 'Q' ")
	ErrBadConnection                   = errors.New("bad connection, server error")
	ErrMaxResponseProtocolSizeExceeded = errors.New("server response protocol max size Exceeded")
	ErrResponseProtocolFormat          = errors.New("server response protocol not start with 'D' and end with 'Z'")
)

type Proxy interface {
	PutClient(server.Client)
	RemoveClient(server.Client)
	GetMaxCount() int
}

type ServerProxy struct {
	dependencies map[server.Client]*Response
	clients      map[server.Client]struct{}
	maxClient    int
	s            server.Server
	lock         sync.Mutex
}

func NewProxy(maxClient int, s server.Server) *ServerProxy {
	return &ServerProxy{
		dependencies: make(map[server.Client]*Response),
		clients:      make(map[server.Client]struct{}),
		maxClient:    maxClient,
		s:            s,
	}
}

func (p *ServerProxy) ClientCount() int {
	return len(p.clients)
}
func (p *ServerProxy) Request(query []byte) (*Response, error) {
	if len(query) == 0 || query[0] != 'Q' {
		return nil, ErrBadRequest
	}
	p.lock.Lock()
	defer p.lock.Unlock()

	client, err := p.getFreeClientLocked()
	if err != nil {
		return nil, err
	}
	return p.createResponseLocked(query, client)
}

func (p *ServerProxy) getFreeClientLocked() (server.Client, error) {

	if client := p.getCacheClientLocked(); client != nil {
		return client, nil
	}
	//more than max clients
	if len(p.clients) >= int(p.maxClient) {
		return nil, ErrClientCountExceeded
	}
	//create new connection
	client, err := p.s.Connect()
	if err != nil {
		return nil, err
	}
	p.clients[client] = struct{}{}
	return client, nil
}

func (p *ServerProxy) getCacheClientLocked() server.Client {
	if len(p.clients) == 0 {
		return nil
	}
	for client, _ := range p.clients {
		if _, exists := p.dependencies[client]; !exists {
			return client
		}
	}
	return nil
}
func (p *ServerProxy) createResponseLocked(query []byte, client server.Client) (*Response, error) {
	err := client.Request(query)
	if err != nil {
		p.clearClientLocked(client)
		//此处如果支持多次尝试，返回一个固定类型的错误，让上层判断是否需要重试，这里返回ErrBadConnection,上层基于这个做判断，目前不做重试
		//TODO 基于错误类型做判断
		return nil, fmt.Errorf("%s[%w]", err.Error(), ErrBadConnection)
	}

	response := NewResponse(client, p)
	p.dependencies[client] = response
	return response, nil
}

func (p *ServerProxy) clearClientLocked(client server.Client) {
	if _, exists := p.clients[client]; exists {
		delete(p.clients, client)
	}
	if _, exists := p.dependencies[client]; exists {
		delete(p.dependencies, client)
	}
}

func (p *ServerProxy) PutClient(client server.Client) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, exists := p.dependencies[client]; exists {
		delete(p.dependencies, client)
	}
}

func (p *ServerProxy) RemoveClient(client server.Client) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, exists := p.dependencies[client]; exists {
		delete(p.dependencies, client)
	}
	if _, exists := p.clients[client]; exists {
		delete(p.clients, client)
	}
}

func (p *ServerProxy) GetMaxCount() int {
	return p.maxClient
}
