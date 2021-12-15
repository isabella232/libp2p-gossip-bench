package toxiproxy_test

import (
	"net"
	"testing"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/testhelper"
)

func NewTestProxy(name, upstream string) *toxiproxy.Proxy {
	proxy := toxiproxy.NewProxy()

	proxy.Name = name
	proxy.Listen = "localhost:0"
	proxy.Upstream = upstream

	return proxy
}

func WithTCPProxy(
	t *testing.T,
	f func(proxy net.Conn, response chan []byte, proxyServer *toxiproxy.Proxy),
) {
	testhelper.WithTCPServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		proxy.Start()

		conn := AssertProxyUp(t, proxy.Listen, true)

		f(conn, response, proxy)

		proxy.Stop()
	})
}

func AssertProxyUp(t *testing.T, addr string, up bool) net.Conn {
	conn, err := net.Dial("tcp", addr)
	if err != nil && up {
		t.Error("Expected proxy to be up:", err)
	} else if err == nil && !up {
		t.Error("Expected proxy to be down")
	}
	return conn
}
