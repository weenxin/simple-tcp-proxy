package proxy

import (
	"github.com/weenxin/simple-tcp-proxy/server"
	"io"
	"sync"
)

const (
	MaxProtocolLength = 5120
)

type Response struct {
	client          server.Client
	parent          Proxy
	data            []byte
	preProtocolSize int
	isClosed        bool
}

//降低垃圾回收频率，我们使用pool，每个P一个Pool，自动伸缩
var dataPoll = sync.Pool{
	New: func() any {
		data := make([]byte, 0, MaxProtocolLength)
		return data
	},
}

func NewResponse(client server.Client, parent Proxy) *Response {
	return &Response{
		client: client,
		parent: parent,
		//从pool中取得，需要设置index为0
		data: dataPoll.Get().([]byte)[:0],
	}
}

func (r *Response) IsClosed() bool {
	return r.isClosed
}

//Read从缓存中读取数据
//WARNING： 返回的数据是共享缓存的，不应该在上面做任何修改，如果需要修改数据，应该单独copy出来做修改
func (r *Response) Read() ([]byte, error) {
	if r.isClosed {
		return nil, io.EOF
	}
	//清空上一帧的缓存
	if r.preProtocolSize > 0 {
		copy(r.data[0:], r.data[r.preProtocolSize:])
		r.data = r.data[0 : len(r.data)-r.preProtocolSize]
		r.preProtocolSize = 0
	}
	//如果TCP粘包导致了收到下一帧的数据
	if len(r.data) > 0 {
		//寻找protocol
		protocol, err := FormatRequest(r.data)
		if err != nil {
			r.removeClient()
			return nil, err
		}
		//fmt.Println(string(protocol))
		if IsEndResponse(protocol) {
			//设置为空闲
			r.putClient()
			return nil, io.EOF
		}
		//找到了
		if protocol != nil {
			r.preProtocolSize = len(protocol)
			return protocol, nil
		}
	}
	//继续读取数据
	readSize, err := r.client.Read(r.data[len(r.data):MaxProtocolLength])
	if err != nil {
		r.removeClient()
		return nil, err
	}
	r.data = r.data[:len(r.data)+readSize]

	return r.Read()
}

func (r *Response) removeClient() {
	//设置为空闲
	r.parent.RemoveClient(r.client)
	//归还buffer
	dataPoll.Put(r.data)
	r.isClosed = true
}

func (r *Response) putClient() {
	//设置为空闲
	r.parent.PutClient(r.client)
	//归还buffer
	dataPoll.Put(r.data)
	r.isClosed = true
}

func (r *Response) Close() error {
	if r.IsClosed() {
		return nil
	}
	for {
		// TODO 增加超时时间
		length, err := r.client.Read(r.data[0:MaxProtocolLength])
		if err != nil {
			r.removeClient()
			return err
		}
		if length > 0 && r.data[length-1] == 'Z' {
			r.putClient()
			return nil
		}
	}
}
