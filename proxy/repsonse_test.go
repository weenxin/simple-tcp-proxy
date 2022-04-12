package proxy_test

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/weenxin/simple-tcp-proxy/proxy"
	"github.com/weenxin/simple-tcp-proxy/server"
	"io"
)

const (
	clientStatusOpen clientStatus = iota
	clientStatusShouldNeverRead
	clientStatusFailed
)

var _ = ginkgo.Describe("Repsonse", func() {

	ginkgo.Describe("Read single protocol", ginkgo.Ordered, func() {
		var response *proxy.Response
		var client *mockStringsClient
		var tmpProxy *mockProxy
		ginkgo.BeforeAll(func() {
			client = &mockStringsClient{
				protocols: [][]byte{[]byte("Daldfjsajldfjaljlljlsjlj"), []byte("Z")},
				index:     0,
			}
			tmpProxy = &mockProxy{
				clients: make(map[server.Client]bool),
			}
			response = proxy.NewResponse(client, tmpProxy)
		})

		ginkgo.When("return a normal response and a end protocol", func() {
			ginkgo.It("get the normal response and then return read EOF", func() {

				ginkgo.By("get the normal response")
				data, err := response.Read()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(string(data)).To(gomega.Equal("Daldfjsajldfjaljlljlsjlj"))
				gomega.Expect(response.IsClosed()).To(gomega.Equal(false))

				ginkgo.By("and then return read EOF")
				data, err = response.Read()
				gomega.Expect(err).To(gomega.Equal(io.EOF))
				gomega.Expect(data).To(gomega.BeNil())
				gomega.Expect(response.IsClosed()).To(gomega.Equal(true))

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

	ginkgo.Describe("read multiple protocol", ginkgo.Ordered, func() {
		var response *proxy.Response
		var client *mockStringsClient
		var tmpProxy *mockProxy
		ginkgo.BeforeAll(func() {
			client = &mockStringsClient{
				protocols: [][]byte{[]byte("DaaaaaaaaaDbbbbbbbb"), []byte("Z")},
				index:     0,
			}
			tmpProxy = &mockProxy{
				clients: make(map[server.Client]bool),
			}
			response = proxy.NewResponse(client, tmpProxy)
		})

		ginkgo.Describe("read message protocol with two protocol", func() {
			ginkgo.It("first time return the first protocol", func() {
				data, err := response.Read()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(string(data)).To(gomega.Equal("Daaaaaaaaa"))
				gomega.Expect(response.IsClosed()).To(gomega.Equal(false))
			})
			ginkgo.It("secold time return the second protocol", func() {
				data, err := response.Read()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(string(data)).To(gomega.Equal("Dbbbbbbbb"))
				gomega.Expect(response.IsClosed()).To(gomega.Equal(false))
			})
			ginkgo.It("return nil data, return io.EOF", func() {
				data, err := response.Read()
				ginkgo.By("return nil data ")
				gomega.Expect(err).To(gomega.Equal(io.EOF))
				ginkgo.By("io.EOF")
				gomega.Expect(data).To(gomega.BeNil())

				ginkgo.By("should recycle client to proxy")
				gomega.Expect(len(tmpProxy.clients)).To(gomega.Equal(1))
				for _, value := range tmpProxy.clients {
					gomega.Expect(value).To(gomega.Equal(true))
				}
			})

		})
	})

	ginkgo.Describe("close a response with data", func() {

		ginkgo.Describe("close a closed response ", func() {
			var response *proxy.Response
			var client *mockStringsClient
			var tmpProxy *mockProxy
			ginkgo.BeforeEach(func() {
				client = &mockStringsClient{
					protocols: [][]byte{[]byte("Daaaaaaaaa"), []byte("Z")},
					index:     0,
				}
				tmpProxy = &mockProxy{
					clients: make(map[server.Client]bool),
				}
				response = proxy.NewResponse(client, tmpProxy)
				data, err := response.Read()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(string(data)).To(gomega.Equal("Daaaaaaaaa"))
				gomega.Expect(response.IsClosed()).To(gomega.Equal(false))

				data, err = response.Read()
				ginkgo.By("return nil data ")
				gomega.Expect(err).To(gomega.Equal(io.EOF))
				ginkgo.By("io.EOF")
				gomega.Expect(data).To(gomega.BeNil())

				err = response.Close()
				gomega.Expect(err).To(gomega.BeNil())
			})
		})

		ginkgo.Describe("close a opened response ", func() {
			var response *proxy.Response
			var client *mockStringsClient
			var tmpProxy *mockProxy
			ginkgo.BeforeEach(func() {
				client = &mockStringsClient{
					protocols: [][]byte{[]byte("Daaaaaaaaa"), []byte("Z")},
					index:     0,
				}
				tmpProxy = &mockProxy{
					clients: make(map[server.Client]bool),
				}
				response = proxy.NewResponse(client, tmpProxy)
				err := response.Close()
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(client.index).To(gomega.Equal(len(client.protocols)))
			})
		})

	})
})
