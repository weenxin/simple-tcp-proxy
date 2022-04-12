# simple-tcp-proxy

## 需求整理
一个简单的 `TCP Proxy` ，需求如下：
- 接收用户
- 如果接收到Client端的请求以'Q'开头则接收数据；否则丢弃数据；
- Server端接收到请求后开始回复数据：
  - 收到Server端的信息，如果以'D'开头则直接转发给Client
  - 收到Server端的信息，如果以'Z'开头则转发给clint，并设置为结束状态；（只有一个字节）


### 分析

#### 1， 接收用户请求

结论：**假设没有完整接收到用户请求前，请求应该是无效的**

***用户请求数据包有多大？***
对于一个db应用，数据插入操作，批次插入数据量会比较大，假设成立？
- 这种情况可以`PrepareStatement`这种方式解决，先`PrepareStatement`返回一个Id，然后基于Id和数据流做batch，分batch发送。与假设不冲突/

***是否有TCP粘包的问题？**

结论：**Client不需要处理TCP粘包的问题**

Server端与Client端是一个单双工的工作方式：
- client端先发送数据；
- server端接收到请求，处理请求，多次返回结果（TCP粘包）；
- client再次发送请求；

所以从Proxy端来看，对于Client端，不需要处理TCP粘包，如果需要处理，我们需要重新设计Client端的Protocol。


#### 2.处理Server端返回；

结论： ***需要连接池支持，不能每个请求单独建立一个链接***

如果我们每次对一个请求创建一个链接，在高并发的场景下，很容易把server端的端口占用满，也失去了Proxy的意义。


结论：**Server端的返回信息要及时返回给client端，方便用户分批处理数据，降低Proxy内存压力**

- 每次返回的Protocol行（我们按照D开头作为开始标记，在TCP粘包的情况下似乎是有冲突的，我们不去质疑Protocol的设计），需要单独返回，因为每次返回的信息可能是一个batch（比如数据库中的一个行等信息），有两种策略，我们选择第二种
  - client端自己处理粘包的问题，proxy只负责收发；（性能更好）
  - **proxy端负责处理粘包**，client端迭代过程中，每次收取的都是标准Protocol。这种方式下，client端不需要处理粘包，每次接收完一个Protocol之后，才会接收第二个protocol。（应用测感知更好）
- 如果不能及时将数据返回给client端处理，会导致proxy端为了接收全量数据（数据大小未知）内存膨胀；如果有分批处理逻辑，是有解决办法的。


#### 3.Server Client重用


结论： **传输过程中，如果有异常，放弃当前链接，并返回给用户错误，让用户重试**

当Server端重启，或者链路中异常点，时应该及时回收链接。当用户再次请求时，新建链接。策略多种多样，一种比较好的方式是，当发生错误时，按照连接池最大值，使用新连接填补连接池。这样做，可以降低对于用户的影响。

结论：**用户读取完所有数据（Z开头的protocol），表示该链接已经使用完成，可以再次复用**

要做连接复用，连接的状态转换可以通过以下方式判断：
- 用户是否读取完成所有数据
- 用户主动`Close`【Optional，增加工时可以解决】


## 模型抽象

<!-- 聚焦在用户使用测 -->
<!-- - ServerClient 代表系统后端，直接连接proxy -->

聚焦在Proxy端，有以下几种组件组成：
- Request （用户请求）
- Client（与真实Server端的连接）
- Response（一个Protocol的返回）

<!-- 聚焦在Server端 -->
<!-- - Server 处理请求，返回数据； -->


## Behavior Driven Development
我们使用行为驱动开发模式，整理行为如下：


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
      - when: "return a normal response and a end protocol"
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



