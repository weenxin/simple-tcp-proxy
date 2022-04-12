package proxy_test

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/weenxin/simple-tcp-proxy/proxy"
	"github.com/weenxin/simple-tcp-proxy/server"
	"io"
	"sync"
)

var _ = ginkgo.Describe("Proxy", func() {
	var s server.Server
	var p *proxy.ServerProxy

	ginkgo.Describe("Request", func() {

		ginkgo.When("request not start with 'Q'", func() {
			ginkgo.BeforeEach(func() {
				s = &mockProxyServer{}
				p = proxy.NewProxy(5, s)
			})
			ginkgo.It("return bad protocol error", func() {
				resonse, err := p.Request([]byte("aaaaaa"))
				gomega.Expect(err).To(gomega.Equal(proxy.ErrBadRequest))
				gomega.Expect(resonse).To(gomega.BeNil())
			})
		})

		ginkgo.Describe("server response a single protocol not start with 'D'", func() {
			ginkgo.BeforeEach(func() {
				s = &mockProxyServer{
					response: [][]byte{[]byte("lsjflfajsdj")},
				}
				p = proxy.NewProxy(5, s)
			})

			ginkgo.It("close connection and return server protocol not matched", func() {
				response, err := p.Request([]byte("Qaafdas"))
				gomega.Expect(err).To(gomega.BeNil())
				protocol, err := response.Read()
				ginkgo.By("return server protocol not matched error")
				gomega.Expect(err).To(gomega.Equal(proxy.ErrResponseProtocolFormat))
				ginkgo.By("protocol is nil")
				gomega.Expect(len(protocol)).To(gomega.Equal(0))
				ginkgo.By("client is empty")
				gomega.Expect(p.ClientCount()).To(gomega.Equal(0))

			})
		})

		ginkgo.Describe("server return half of protocol not to end", func() {
			ginkgo.BeforeEach(func() {
				s = &mockProxyServer{
					response: [][]byte{[]byte("Dsjflfajsdj")},
				}
				p = proxy.NewProxy(5, s)
			})

			ginkgo.It("close connection and return server protocol not matched", func() {
				response, err := p.Request([]byte("Qaafdas"))
				gomega.Expect(err).To(gomega.BeNil())
				protocol, err := response.Read()
				ginkgo.By("return server protocol not matched error")
				gomega.Expect(err).To(gomega.Equal(io.EOF))
				ginkgo.By("protocol is nil")
				gomega.Expect(len(protocol)).To(gomega.Equal(0))
				ginkgo.By("client is empty")
				gomega.Expect(p.ClientCount()).To(gomega.Equal(0))

			})
		})

		ginkgo.Describe("server response a single protocol start with 'D'", func() {
			ginkgo.BeforeEach(func() {
				s = &mockProxyServer{
					response: [][]byte{[]byte("Daaaaa"), []byte("Z")},
				}
				p = proxy.NewProxy(5, s)
			})

			ginkgo.It("close connection and return server protocol not matched", func() {
				response, err := p.Request([]byte("Qaafdas"))
				gomega.Expect(err).To(gomega.BeNil())
				protocol, err := response.Read()
				ginkgo.By("return no error")
				gomega.Expect(err).To(gomega.BeNil())
				ginkgo.By("protocol is normal")
				gomega.Expect(string(protocol)).To(gomega.Equal("Daaaaa"))

				ginkgo.By("read Z")
				protocol, err = response.Read()
				ginkgo.By("return io.EOF")
				gomega.Expect(err).To(gomega.Equal(io.EOF))
				ginkgo.By("protocol is normal")
				gomega.Expect(len(protocol)).To(gomega.Equal(0))
				ginkgo.By("client is not empty")
				gomega.Expect(response.IsClosed()).To(gomega.Equal(true))

			})
		})

		ginkgo.Describe("server response a single protocol start with 'D'", func() {
			ginkgo.BeforeEach(func() {
				s = &mockProxyServer{
					response: [][]byte{[]byte("Daaaaa"), []byte("Z")},
				}
				p = proxy.NewProxy(5, s)
			})

			ginkgo.It("return no error & protocol is normal & client is 1", func() {
				response, err := p.Request([]byte("Qaafdas"))
				gomega.Expect(err).To(gomega.BeNil())
				protocol, err := response.Read()
				ginkgo.By("return no error")
				gomega.Expect(err).To(gomega.BeNil())
				ginkgo.By("protocol is normal")
				gomega.Expect(string(protocol)).To(gomega.Equal("Daaaaa"))
				gomega.Expect(p.ClientCount()).To(gomega.Equal(1))

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

		ginkgo.Describe("server response multiple protocol in one line", func() {
			ginkgo.BeforeEach(func() {
				s = &mockProxyServer{
					response: [][]byte{[]byte("DaaaaaaaaaDbbbbbbbb"), []byte("Z")},
				}
				p = proxy.NewProxy(5, s)
			})

			ginkgo.It("return normal case", func() {
				response, err := p.Request([]byte("Qaafdas"))
				gomega.Expect(err).To(gomega.BeNil())
				protocol, err := response.Read()
				ginkgo.By("return no error")
				gomega.Expect(err).To(gomega.BeNil())
				ginkgo.By("protocol is normal")
				gomega.Expect(string(protocol)).To(gomega.Equal("Daaaaaaaaa"))
				gomega.Expect(p.ClientCount()).To(gomega.Equal(1))

				protocol, err = response.Read()
				ginkgo.By("return no error")
				gomega.Expect(err).To(gomega.BeNil())
				ginkgo.By("protocol is normal")
				gomega.Expect(string(protocol)).To(gomega.Equal("Dbbbbbbbb"))
				gomega.Expect(p.ClientCount()).To(gomega.Equal(1))

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

		ginkgo.Describe("reuse a client, serial request to proxy two time ", func() {
			ginkgo.BeforeEach(func() {
				s = &mockProxyServer{
					response: [][]byte{
						[]byte("Daaaaaaaaa"), []byte("Z"),
						[]byte("Dcccccccc"), []byte("Z"),
					},
				}
				p = proxy.NewProxy(5, s)
			})

			ginkgo.It("close connection and return server protocol not matched", func() {
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

				gomega.Expect(p.ClientCount()).To(gomega.Equal(1))
			})
		})

		ginkgo.Describe("create more than one connection, but no more than max connection", func() {
			ginkgo.BeforeEach(func() {
				s = &mockProxyServer{
					response: [][]byte{
						[]byte("Daaaaaaaaa"), []byte("Z"),
					},
				}
				p = proxy.NewProxy(5, s)
			})

			ginkgo.It("no more than max connection and connection reused", func() {

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

				response, err := p.Request([]byte("Qxxxxx"))
				gomega.Expect(err).To(gomega.Equal(proxy.ErrClientCountExceeded))
				gomega.Expect(response).To(gomega.BeNil())

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

				response, err = p.Request([]byte("Qxxxxx"))
				gomega.Expect(err).To(gomega.BeNil())

			})
		})

	})

})
