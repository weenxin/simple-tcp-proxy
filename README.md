# simple-tcp-proxy

## 需求整理
一个简单的 `TCP Proxy` ，需求如下：
- 接收用户
- 如果接收到Client端的请求以'Q'开头则接收数据；否则丢弃数据；
- Server端接收到请求后开始回复数据：
  - 收到Server端的信息，如果以'D'开头则直接转发给Client
  - 收到Server端的信息，如果以'Z'开头结束本次请求；（只有一个字节）



## 模型抽象

<!-- 聚焦在用户使用测 -->
<!-- - ServerClient 代表系统后端，直接连接proxy -->

抽象以下组件：
- Request （用户请求）
- Proxy （接收用户请求）
- Server （Proxy连接server 调用Connect后获取一个Client）
- Client（与真实Server端的连接）
- Response：多个`D`开头的字符序列，直到收到一个`Z`
- Protocol： `D`开头的字符序列

<!-- 聚焦在Server端 -->
<!-- - Server 处理请求，返回数据； -->


### 分析

#### 1.接收用户请求

**用户请求报文大小，可以一次性接收，不需要单独处理**

考虑一个db应用，批次插入的数据量会比较大，一次性接收用户报文是否可行？
- 这种情况可以`PrepareStatement`这种方式解决，先`PrepareStatement`返回一个Id，然后基于Id和数据流做batch，分batch发送。与假设不冲突。

**是否有TCP粘包的问题？**

Server端与Client端是一个单双工的工作方式：
- client端先发送数据；
- server端接收到请求，处理请求，多次返回结果（TCP粘包）；
- client再次发送请求；

所以Proxy端接收Client端请求过程，不需要处理TCP粘包，【如果需要处理，我们需要重新设计Client端的Protocol】。



#### 2.处理Server端返回

**需要连接池支持**

如果每次对一个请求创建一个新的链接，在高并发的场景下，很容易把server端的端口占用满，也失去了Proxy的意义。

**Server端的返回信息要及时返回给client端，方便用户分批处理数据，同时降低Proxy内存压力**

- 每个Protocol行需要尽快返回，因为每个protocol是可以独立解析的，比如一行数据；此时需要处理TCP粘包的问题；
- 不能及时将数据返回给client端，会导致proxy端为了接收大量数据而内存膨胀；



#### 3.Server Client重用

**放弃异常连接，用户重试**

- 当Server端重启时，已经建立的连接会失效，应该及时删除；
- 当用户再次请求时，基于新启动的server重新建立连接，故障自愈。

**复用正常连接**

- 用户读取完成所有数据response后，回收连接；
- 用户主动`Close`连接，应该将server端的未读报文清空，回收连接；



## 使用方法

- `p := proxy.NewProxy` ，新建一个Proxy
-  `resonse,err := p.Request([]byte("Qfist"))` , 发送请求，获得response
- `for protocol, err := response.Read(); err != nil ; {}` 迭代遍历response获得每个protocol;

### 优点
- 调用方无需感知连接池等信息，但确实有连接池
- 服务端重启，自动重新连接



##  结果查看

已经完成了单元测试，[查看运行结果](https://github.com/weenxin/simple-tcp-proxy/actions) 


## 设计与开发

我们使用行为驱动开发模式，模型整理行为如下：

```yaml
given:
  - name: "proxy"
    - module: "Request"
      - when: "request not start with 'Q'"
        - should: "return bad protocol error"

      - when: "server response a single protocol not start with 'D'"
        -  should: "close connection and return server protocol not matched"

      - when: "server return half of protocol not to end"
        - should: "return bad format error"
        - should: "recycle the client"

      - when: "server response a single protocol start with 'D'"
        - should: "return no error"
        - should: "return the normal protocol"
        - should: "proxy client count should be 1"
        - should: "read second time, should return eof"
        - should: "len protocol should be zero"
        - should: "response is closed"

      - when: "server response multiple protocol : first start with 'D' and some of second protocol is in first protcol,that means second protocol not start with 'D' "
        - should: "request return not error and a response"
        - should: "first read response ,retrun no err ,and the first protocol"
        - should: "second read response, return no err, and second protocol"
        - should: "response is closed"
        - should: "proxy client count should be 1"

      - when: "reuse a client, serial request to proxy two time "
        - should:  "request return not error and a response"
        - should: "proxy client count should be 1"

      - when: "create more than one connection, but no more than max connection"
        - should: "request <= max connection and don't read response"
        - should: "all should return a good response , and no error"
        - should: "create a new request , will get MaxConnectionExcceded error"
        - should: "read a good response will get no err ,and right response"
        - should: "create a new request will success"
        - should: "proxy client count should be max connection count"

      - when: "when use has not read all data from a connection and not close it"
        - should: "connection should not be reused"


  - name: "Response"
    - module: "Read"
      - when: "client return a normal response and a end protocol"
        - should: "get the normal response and then return read EOF"
        - should: "response status should be closed"
        - should: "recycle the client to proxy"
      - when: "read a closed response"
        - should: "return nil data"
        - should: "return io.EOF"
      - when: "read message protocol with two protocol"
        - should: "first time return the first protocol"
        - should: "second time return the second protocol"
        - should: "return nil data, return io.EOF, response is closed, client is recycled"
    - module: "Close"
      - when: "close a closed repsonse"
        - should:  "return no err"
      - when: "close a opened repsonse"
        - should:  "return no err"
        - should:  "clear protocol remaining data "

```

## 部分用例说明：

### Response 测试

####  正常读一条记录

```yaml
- name: "Response"
    - module: "Read"
      - when: "client return a normal response and a end protocol"
        - should: "get the normal response and then return read EOF"
        - should: "response status should be closed"
        - should: "recycle the client to proxy"
```

给定一个repsonse，对于它的读函数做测试，当服务端返回一个正常的报文并且返回一个结束时：
- 应该先返回一个正常的报文，然后返回EOF
- response的状态应该是已经关闭的
- client应该是被回收的


测试代码为：
```go

//串行执行所有用例
ginkgo.Describe("Read single protocol", ginkgo.Ordered, func() {
	var response *proxy.Response
	var client *mockStringsClient
	var tmpProxy *mockProxy
	//所有用例前，先执行这个，初始化一次
	ginkgo.BeforeAll(func() {
		client = &mockStringsClient{
		    //第一次读返回server端返回Daldfjsajldfjaljlljlsjlj，第二次返回Z
			protocols: [][]byte{[]byte("Daldfjsajldfjaljlljlsjlj"), []byte("Z")},
			index:     0,
		}
		//记录下proxy的状态
		tmpProxy = &mockProxy{
			clients: make(map[server.Client]bool),
		}
		response = proxy.NewResponse(client, tmpProxy)
	})

    //服务端返回一个正常的报文并且返回一个结束时
	ginkgo.When("client return a normal response and a end protocol", func() {
	    //它应该应该先返回一个正常的报文，然后返回EOF
		ginkgo.It("get the normal response and then return read EOF", func() {
            //正常获取第一条数据
			ginkgo.By("get the normal response")
			data, err := response.Read()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(data)).To(gomega.Equal("Daldfjsajldfjaljlljlsjlj"))
			gomega.Expect(response.IsClosed()).To(gomega.Equal(false))

            //第二次读，返回IO.EOF
			ginkgo.By("and then return read EOF")
			data, err = response.Read()
			gomega.Expect(err).To(gomega.Equal(io.EOF))
			gomega.Expect(data).To(gomega.BeNil())
			gomega.Expect(response.IsClosed()).To(gomega.Equal(true))

            //response被关闭后，应该连接被回收
			ginkgo.By("should recycle client to proxy")
			gomega.Expect(len(tmpProxy.clients)).To(gomega.Equal(1))
			for _, value := range tmpProxy.clients {
				gomega.Expect(value).To(gomega.Equal(true))
			}
		})

	})

	ginkgo.When("read a closed response", func() {
		ginkgo.It("return nil data and return io.EOF", func() {
			ginkgo.By("a closed response")
			gomega.Expect(response.IsClosed()).To(gomega.Equal(true))

			ginkgo.By("return nil data ")
			data, err := response.Read()
			gomega.Expect(err).To(gomega.Equal(io.EOF))
			ginkgo.By("io.EOF")
			gomega.Expect(data).To(gomega.BeNil())

		})
	})
})


```




#### TCP粘包问题解决

```go
//串行执行用例
ginkgo.Describe("read multiple protocol", ginkgo.Ordered, func() {
	var response *proxy.Response
	var client *mockStringsClient
	var tmpProxy *mockProxy

	ginkgo.BeforeAll(func() {
		client = &mockStringsClient{
		    //两个返回包粘包
			protocols: [][]byte{[]byte("DaaaaaaaaaDbbbbbbbb"), []byte("Z")},
			index:     0,
		}
		tmpProxy = &mockProxy{
			clients: make(map[server.Client]bool),
		}
		response = proxy.NewResponse(client, tmpProxy)
	})

	ginkgo.Describe("read message protocol with two protocol", func() {
	//可以接收第一个报文
		ginkgo.It("first time return the first protocol", func() {
			data, err := response.Read()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(data)).To(gomega.Equal("Daaaaaaaaa"))
			gomega.Expect(response.IsClosed()).To(gomega.Equal(false))
		})
		//可以接收第二个报文
		ginkgo.It("secold time return the second protocol", func() {
			data, err := response.Read()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(data)).To(gomega.Equal("Dbbbbbbbb"))
			gomega.Expect(response.IsClosed()).To(gomega.Equal(false))
		})
		//再次读就response就释放了
		ginkgo.It("return nil data, return io.EOF", func() {
			data, err := response.Read()
			ginkgo.By("return nil data ")
			gomega.Expect(err).To(gomega.Equal(io.EOF))
			ginkgo.By("io.EOF")
			gomega.Expect(data).To(gomega.BeNil())

            //连接被回收啦
			ginkgo.By("should recycle client to proxy")
			gomega.Expect(len(tmpProxy.clients)).To(gomega.Equal(1))
			for _, value := range tmpProxy.clients {
				gomega.Expect(value).To(gomega.Equal(true))
			}
		})

	})
})
```


### Proxy测试

#### 报文不符合要求

```go
ginkgo.When("request not start with 'Q'", func() {
	ginkgo.BeforeEach(func() {
		s = &mockProxyServer{}
		p = proxy.NewProxy(5, s)
	})
	ginkgo.It("return bad protocol error", func() {
	    // 一个不以Q开头的protocol
		resonse, err := p.Request([]byte("aaaaaa"))
		//返回ErrBadRequest错误
		gomega.Expect(err).To(gomega.Equal(proxy.ErrBadRequest))
		//返回空response
		gomega.Expect(resonse).To(gomega.BeNil())
	})
})
```


#### 连接数与粘包

```go
//尝试创建多个连接，但是不会超过最大连接数
ginkgo.Describe("create more than one connection, but no more than max connection", func() {
	ginkgo.BeforeEach(func() {
		s = &mockProxyServer{
			response: [][]byte{
			    //都会返回这个结果
				[]byte("Daaaaaaaaa"), []byte("Z"),
			},
		}
		p = proxy.NewProxy(5, s)
	})

//
	ginkgo.It("no more than max connection and connection reused", func() {

        //新建最大连接数
		var wg sync.WaitGroup
		wg.Add(p.GetMaxCount())
		responses := make([]*proxy.Response, p.GetMaxCount())
		for i := 0; i < p.GetMaxCount(); i++ {
			go func(index int) {
				response, err := p.Request([]byte("Qxxxxx"))
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(response).NotTo(gomega.BeNil())
				responses[index] = response
				wg.Done()
			}(i)
		}

		wg.Wait()
		gomega.Expect(p.ClientCount()).To(gomega.Equal(p.GetMaxCount()))

        //再次新建连接失败
		response, err := p.Request([]byte("Qxxxxx"))
		gomega.Expect(err).To(gomega.Equal(proxy.ErrClientCountExceeded))
		gomega.Expect(response).To(gomega.BeNil())

        //释放一个连接
		response = responses[0]
		protocol, err := response.Read()
		ginkgo.By("return no error")
		gomega.Expect(err).To(gomega.BeNil())
		ginkgo.By("protocol is normal")
		gomega.Expect(string(protocol)).To(gomega.Equal("Daaaaaaaaa"))
		protocol, err = response.Read()
		gomega.Expect(err).To(gomega.Equal(io.EOF))
		gomega.Expect(len(protocol)).To(gomega.Equal(0))

		gomega.Expect(p.ClientCount()).To(gomega.Equal(p.GetMaxCount()))

        //可以正常取得连接
		response, err = p.Request([]byte("Qxxxxx"))
		gomega.Expect(err).To(gomega.BeNil())

	})
})
```


#### 粘包测试

```go
//粘包测试
ginkgo.Describe("server response multiple protocol in one line", func() {
	ginkgo.BeforeEach(func() {
		s = &mockProxyServer{
			response: [][]byte{[]byte("DaaaaaaaaaDbbbbbbbb"), []byte("Z")},
		}
		p = proxy.NewProxy(5, s)
	})


	ginkgo.It("return normal case", func() {
	    //第一次读成功
		response, err := p.Request([]byte("Qaafdas"))
		gomega.Expect(err).To(gomega.BeNil())
		protocol, err := response.Read()
		ginkgo.By("return no error")
		gomega.Expect(err).To(gomega.BeNil())
		ginkgo.By("protocol is normal")
		gomega.Expect(string(protocol)).To(gomega.Equal("Daaaaaaaaa"))
		gomega.Expect(p.ClientCount()).To(gomega.Equal(1))

        //第二次读成功
		protocol, err = response.Read()
		ginkgo.By("return no error")
		gomega.Expect(err).To(gomega.BeNil())
		ginkgo.By("protocol is normal")
		gomega.Expect(string(protocol)).To(gomega.Equal("Dbbbbbbbb"))
		gomega.Expect(p.ClientCount()).To(gomega.Equal(1))

        //第三次读取io.EOF,response被关闭
		ginkgo.By("read Z")
		protocol, err = response.Read()
		ginkgo.By("return io.EOF")
		gomega.Expect(err).To(gomega.Equal(io.EOF))
		ginkgo.By("len protocol should be zero")
		gomega.Expect(len(protocol)).To(gomega.Equal(0))
		ginkgo.By("response is closed")
		gomega.Expect(response.IsClosed()).To(gomega.Equal(true))
	})
})
```

#### 连接复用测试

```go
ginkgo.Describe("reuse a client, serial request to proxy two time ", func() {
	ginkgo.BeforeEach(func() {
		s = &mockProxyServer{
		    //一个连接可以服务两次请求
			response: [][]byte{
				[]byte("Daaaaaaaaa"), []byte("Z"),
				[]byte("Dcccccccc"), []byte("Z"),
			},
		}
		p = proxy.NewProxy(5, s)
	})

	ginkgo.It("close connection and return server protocol not matched", func() {
	    //第一次请求
		response, err := p.Request([]byte("Qfirst"))
		gomega.Expect(err).To(gomega.BeNil())
		protocol, err := response.Read()
		ginkgo.By("return no error")
		gomega.Expect(err).To(gomega.BeNil())
		ginkgo.By("protocol is normal")
		gomega.Expect(string(protocol)).To(gomega.Equal("Daaaaaaaaa"))
		gomega.Expect(p.ClientCount()).To(gomega.Equal(1))

		ginkgo.By("read Z")
		protocol, err = response.Read()
		ginkgo.By("return io.EOF")
		gomega.Expect(err).To(gomega.Equal(io.EOF))
		gomega.Expect(p.ClientCount()).To(gomega.Equal(1))

        //第二次请求
		response, err = p.Request([]byte("Qsecond"))
		gomega.Expect(err).To(gomega.BeNil())
		protocol, err = response.Read()
		ginkgo.By("return no error")
		gomega.Expect(err).To(gomega.BeNil())
		ginkgo.By("protocol is normal")
		gomega.Expect(string(protocol)).To(gomega.Equal("Dcccccccc"))
		gomega.Expect(p.ClientCount()).To(gomega.Equal(1))

		ginkgo.By("read Z")
		protocol, err = response.Read()
		ginkgo.By("return io.EOF")
		gomega.Expect(err).To(gomega.Equal(io.EOF))
		ginkgo.By("len protocol should be zero")
		gomega.Expect(len(protocol)).To(gomega.Equal(0))
		ginkgo.By("response is closed")
		gomega.Expect(response.IsClosed()).To(gomega.Equal(true))

        //只有一个连接
		gomega.Expect(p.ClientCount()).To(gomega.Equal(1))
	})
})
```



## 实现细节


#### 内存池连接池

保证有多少的最大连接，就分配多大的内存，内存可控，垃圾回收友好；

```go
//降低垃圾回收频率，我们使用pool，每个P一个Pool，自动伸缩
var dataPoll = sync.Pool{
	New: func() any {
		return make([]byte, 0, MaxProtocolLength)
	},
}
```

新建response
```go
func NewResponse(client server.Client, parent Proxy) *Response {
	return &Response{
		client: client,
		parent: parent,
		//从pool中取得，复用缓存
		data: dataPoll.Get().([]byte)[:0],
	}
}
```

归还内存

```go
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
```

#### 多帧复用
一个Response读取过程中，只使用一片内存，没有多余内存分配。
```
//resonse.go：Read

//清空上一帧的缓存
	if r.preProtocolSize > 0 {
		copy(r.data[0:], r.data[r.preProtocolSize:])
		r.data = r.data[0 : len(r.data)-r.preProtocolSize]
		r.preProtocolSize = 0
	}
```

#### 所有需要加锁才能调用的函数以Locked结尾
- createResponseLocked
- getCacheClientLocked
- deleteClientLocked


