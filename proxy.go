package antch

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
)

// ProxyKey is a key for the proxy URL that used by Crawler.
type ProxyKey struct{}

func proxyHandler(f func(*http.Request) (*url.URL, error), next HttpMessageHandler) HttpMessageHandler {
	// Registers proxy protocol(HTTP,HTTPS,SOCKS5).
	proxy.RegisterDialerType("http", httpProxy)
	proxy.RegisterDialerType("https", httpProxy)

	return HttpMessageHandlerFunc(func(req *http.Request) (*http.Response, error) {
		proxyURL, err := f(req)
		if err != nil {
			return nil, err
		}
		ctx := context.WithValue(req.Context(), ProxyKey{}, proxyURL)
		return next.Send(req.WithContext(ctx))
	})
}

var zeroDialer net.Dialer

func proxyDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if v := ctx.Value(ProxyKey{}); v != nil {
		dialer, err := proxy.FromURL(v.(*url.URL), proxy.Direct)
		if err != nil {
			return nil, err
		}
		return dialer.Dial(network, address)
	}
	return zeroDialer.DialContext(ctx, network, address)
}

func httpProxy(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	h := &httpDialer{
		host:    u.Host,
		forward: forward,
	}
	if u.User != nil {
		h.shouldAuth = true
		h.username = u.User.Username()
		h.password, _ = u.User.Password()
	}
	return h, nil
}

type httpDialer struct {
	host               string
	shouldAuth         bool
	username, password string

	forward proxy.Dialer
}

func (d *httpDialer) auth() string {
	if d.shouldAuth {
		auth := d.username + ":" + d.password
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	}
	return ""
}

func (d *httpDialer) Dial(network, addr string) (net.Conn, error) {
	conn, err := d.forward.Dial("tcp", d.host)
	if err != nil {
		return nil, err
	}

	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
		Close:  false,
	}
	if pa := d.auth(); pa != "" {
		connectReq.Header.Set("Proxy-Authorization", pa)
	}
	connectReq.Write(conn)

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		conn.Close()
		return nil, err
	}

	defer resp.Body.Close()
	io.Copy(ioutil.Discard, resp.Body)

	if resp.StatusCode != 200 {
		conn.Close()
		f := strings.SplitN(resp.Status, " ", 2)
		return nil, fmt.Errorf("proxy: %v", errors.New(f[1]))
	}

	return conn, nil
}

// ProxyMiddleware is an HTTP proxy middleware to take HTTP Request
// use the HTTP proxy to access remote sites.
//
// ProxyMiddleware supports HTTP/HTTPS,SOCKS5 protocol list.
// etc http://127.0.0.1:8080 or https://127.0.0.1:8080 or socks5://127.0.0.1:1080
func ProxyMiddleware(f func(*http.Request) (*url.URL, error)) Middleware {
	return func(next HttpMessageHandler) HttpMessageHandler {
		return proxyHandler(f, next)
	}
}
