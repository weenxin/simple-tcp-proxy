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

//Proxy 是proxy的性能抽象，部分接口没有开放，可以按需开放
type Proxy interface {
	//PutClient 回收client
	PutClient(server.Client)
	//RemoveClient 删除client
	RemoveClient(server.Client)
	//GetMaxCount 获得最大连接数
	GetMaxCount() int
	//Request 新建请求
	Request(query []byte) (*Response, error)
}

//ServerProxy 一份Proxy实例
type ServerProxy struct {
	//是否有client依赖，client在dependencies存在时表示有Response依赖与它，不能复用；当Response全部读取完成后自动释放依赖
	dependencies map[server.Client]*Response
	//当前的连接
	clients map[server.Client]struct{}
	//最大连接数
	maxClient int
	//后端的server
	s server.Server
	//锁
	lock sync.Mutex
}

func NewProxy(maxClient int, s server.Server) *ServerProxy {
	return &ServerProxy{
		dependencies: make(map[server.Client]*Response),
		clients:      make(map[server.Client]struct{}),
		maxClient:    maxClient,
		s:            s,
	}
}

//ClientCount 最大连接数
func (p *ServerProxy) ClientCount() int {
	return len(p.clients)
}

//Request 请求
func (p *ServerProxy) Request(query []byte) (*Response, error) {
	//request判断
	if len(query) == 0 || !IsGoodRequest(query) {
		return nil, ErrBadRequest
	}
	p.lock.Lock()
	defer p.lock.Unlock()

	//获取一个空闲连接，在没有超过最大连接数的情况下，如果当前没有空闲连接，会从server端新建
	client, err := p.getFreeClientLocked()
	if err != nil {
		return nil, err
	}
	//创建response并记录依赖
	return p.createResponseLocked(query, client)
}

func (p *ServerProxy) getFreeClientLocked() (server.Client, error) {

	//先看看缓存中是否有空闲的
	if client := p.getCacheClientLocked(); client != nil {
		return client, nil
	}
	//超出连接数
	if len(p.clients) >= int(p.maxClient) {
		return nil, ErrClientCountExceeded
	}
	//没有空闲连接，并且没有超出最大连接数，新建连接
	client, err := p.s.Connect()
	if err != nil {
		return nil, err
	}
	p.clients[client] = struct{}{}
	return client, nil
}

//getCacheClientLocked 从cache中获取
func (p *ServerProxy) getCacheClientLocked() server.Client {
	if len(p.clients) == 0 {
		return nil
	}
	//在client中
	for client, _ := range p.clients {
		//但是没有依赖
		if _, exists := p.dependencies[client]; !exists {
			return client
		}
	}
	return nil
}
func (p *ServerProxy) createResponseLocked(query []byte, client server.Client) (*Response, error) {
	err := client.Request(query)
	//如果请求失败了，连接可能有问题丢弃连接
	if err != nil {
		p.deleteClientLocked(client)
		//此处如果支持多次尝试，返回一个固定类型的错误，让上层判断是否需要重试，这里返回ErrBadConnection,上层基于这个做判断，目前不做重试
		//TODO 基于错误类型做判断
		return nil, fmt.Errorf("%s[%w]", err.Error(), ErrBadConnection)
	}

	response := NewResponse(client, p)
	//占用一个连接
	p.dependencies[client] = response
	return response, nil
}

//deleteClientLocked 删除连接
func (p *ServerProxy) deleteClientLocked(client server.Client) {
	if _, exists := p.clients[client]; exists {
		delete(p.clients, client)
	}
	if _, exists := p.dependencies[client]; exists {
		delete(p.dependencies, client)
	}
}

//PutClient 回收连接
func (p *ServerProxy) PutClient(client server.Client) {
	p.lock.Lock()
	defer p.lock.Unlock()
	//删除依赖就好
	if _, exists := p.dependencies[client]; exists {
		delete(p.dependencies, client)
	}
}

//RemoveClient 删除连接
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

//GetMaxCount 获取最大连接数
func (p *ServerProxy) GetMaxCount() int {
	//TODO 更改maxClient为int32, 然后使用atomic获取，当前没有设置逻辑，所以还好
	return p.maxClient
}
