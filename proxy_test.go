package antch

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"testing"
)

func testProxyHandler(t *testing.T, proxyURL *url.URL) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	}))
	defer ts.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: proxyDialContext,
		},
	}

	handler := proxyHandler(http.ProxyURL(proxyURL), backMessageHandler(client))
	req, _ := http.NewRequest("GET", ts.URL, nil)
	resp, err := handler.Send(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if g, e := string(b), "hello world"; g != e {
		t.Error("expected %s; got %s", e, g)
	}
}
func TestHTTPProxyHandler(t *testing.T) {
	// HTTP proxy server.
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		director := func(req *http.Request) {
			req.URL.Host = r.Host
			req.URL.Scheme = "http"
		}
		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(w, r)
	}))
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)
	testProxyHandler(t, proxyURL)
}

func _TestSOCKS5ProxyHandler(t *testing.T) {
	gateway, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer gateway.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Listening and accepting incomeing connection.
		c, err := gateway.Accept()
		if err != nil {
			t.Errorf("net.Listener.Accept failed: %v", err)
			return
		}
		var (
			b = make([]byte, 32)
			n = 3
		)
		n, _ = io.ReadFull(c, b[:n])

		c.Close()
	}()

	proxyURL, _ := url.Parse(fmt.Sprintf("socks5://%s", gateway.Addr()))
	testProxyHandler(t, proxyURL)
	wg.Wait()
}
