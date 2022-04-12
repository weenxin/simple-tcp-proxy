package proxy

import (
	"github.com/weenxin/simple-tcp-proxy/server"
	"io"
	"sync"
)

const (
	//MaxProtocolLength 一个Protocol最大5120。一个response可以由多个protocol组成，所以假设还算合理
	MaxProtocolLength = 5120
)

//Response 代表一个request的返回，一个Response 由多个Protocol组成，也就是以`D`分割的多行
type Response struct {
	//server端连接
	client server.Client
	//父亲
	parent Proxy
	//数据缓存
	data []byte
	//上一次接收的大小
	preProtocolSize int
	//是否已经被关闭
	isClosed bool
}

//降低垃圾回收频率，我们使用pool，每个P一个Pool，自动伸缩
//理论情况下，有多少个最大连接数，就会有多少个缓存对象，所以内存应该是 MaxProtocolLength * maxConnection
var dataPoll = sync.Pool{
	New: func() any {
		return make([]byte, 0, MaxProtocolLength)
	},
}

//NewResponse 新建一个response，后续可以基于此进行迭代了
func NewResponse(client server.Client, parent Proxy) *Response {
	return &Response{
		client: client,
		parent: parent,
		//从pool中取得，复用缓存
		data: dataPoll.Get().([]byte)[:0],
	}
}

//IsClosed 是否被关闭，当用户读取完最后一个protocol（读取完`Z`）后自动关闭，否则就是用户主动关闭，需要清空连接内的残留报文
func (r *Response) IsClosed() bool {
	return r.isClosed
}

//Read从缓存中读取数据
//WARNING： 返回的数据是共享缓存的，不应该在上面做任何修改，如果需要修改数据，应该单独copy出来做修改
func (r *Response) Read() ([]byte, error) {
	//如果一斤关闭
	if r.isClosed {
		return nil, io.EOF
	}
	//复用缓存区，清空上一帧的缓存，可以做环形队列，但要处理接收异常；我们的策略是这样，效率也可以，只是需要copy下内存
	if r.preProtocolSize > 0 {
		copy(r.data[0:], r.data[r.preProtocolSize:])
		r.data = r.data[0 : len(r.data)-r.preProtocolSize]
		r.preProtocolSize = 0
	}
	//如果TCP粘包导致了收到下一帧的数据
	if len(r.data) > 0 {
		//从数据中获取一个protocol
		protocol, err := FormatProtocol(r.data)
		if err != nil {
			r.removeClient()
			return nil, err
		}
		//是否是最后一帧啦
		if IsEndResponse(protocol) {
			//回收连接
			r.putClient()
			return nil, io.EOF
		}
		//找到一个protocol
		if protocol != nil {
			r.preProtocolSize = len(protocol)
			return protocol, nil
		}
	}
	//没有数据，就先读取数据
	readSize, err := r.client.Read(r.data[len(r.data):MaxProtocolLength])
	//读取失败，连接有问题，应该删除连接
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
		// TODO Read增加超时时间，否则会被动hang在这里
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
