package proxy_test

import (
	"errors"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/weenxin/simple-tcp-proxy/proxy"
	"github.com/weenxin/simple-tcp-proxy/server"
	"io"
)

var errClientFailed = errors.New("client failed for unknown reason")

type clientStatus int8

//mockProxyServer mock一个server
type mockProxyServer struct {
	clients  []*mockStringsClient //连接数量
	response [][]byte             //每个连接都发送同一样的处理逻辑
}

//Connect 创建一个连接，这个连接的读只会发送server的response数据
func (s *mockProxyServer) Connect() (server.Client, error) {
	client := mockStringsClient{status: clientStatusOpen, protocols: s.response}
	s.clients = append(s.clients, &client)
	return &client, nil
}

//mockStringsClient mock一个client
type mockStringsClient struct {
	protocols [][]byte     //每次读都会读到一个条目
	status    clientStatus //暂时没有用起来，用来模拟服务端异常的，设置为failed就不能读到信息，返回错误了
	index     int          //第几条数据该返回了
}

// 对所有请求都一样长距离，Protocol的有效性由proxy来验证
func (f mockStringsClient) Request([]byte) error {
	return nil
}

// Read， 返回数据
func (f *mockStringsClient) Read(data []byte) (int, error) {

	if f.status == clientStatusFailed {
		return 0, errClientFailed
	} else if f.status == clientStatusShouldNeverRead {
		ginkgo.Fail("should never be read")
	}
	if f.index == len(f.protocols) {
		return 0, io.EOF
	}

	length := copy(data, f.protocols[f.index])
	gomega.Expect(length).To(gomega.Equal(len(f.protocols[f.index])))
	f.index++
	return length, nil
}

//mockProxy 用来测试response对象行为
type mockProxy struct {
	clients map[server.Client]bool
}

func (p mockProxy) Request(query []byte) (*proxy.Response, error) {
	return nil, nil
}

func (p mockProxy) PutClient(client server.Client) {
	p.clients[client] = true
}

func (p mockProxy) RemoveClient(client server.Client) {
	delete(p.clients, client)
}
func (p mockProxy) GetMaxCount() int {
	//无所谓的，只是用来测试response
	return 1000
}
