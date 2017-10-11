package middleware

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
	"sync"

	"github.com/antchfx/antch"
	"golang.org/x/net/proxy"
)

// Proxy is a middleware of Downloader allows all HTTP requests
// access internet via proxy server.
// Proxy middleware supports the following proxy protocols:
// HTTP, HTTPS, SOCKS5.
//
type Proxy struct {
	// ProxyURL specifies a function to return a proxy for a given
	// Request.
	ProxyURL func(*http.Request) *url.URL

	Next antch.Downloader

	once sync.Once
}

// ProxyKey is a proxy key for HTTP request context that used by
// proxy middleware.
type ProxyKey struct{}

var zero net.Dialer

func (p *Proxy) dial(ctx context.Context, network, address string) (net.Conn, error) {
	if v := ctx.Value(ProxyKey{}); v != nil {
		dialer, err := proxy.FromURL(v.(*url.URL), proxy.Direct)
		if err != nil {
			return nil, err
		}
		return dialer.Dial(network, address)
	}
	return zero.DialContext(ctx, network, address)
}

func (p *Proxy) ProcessRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	p.once.Do(func() {
		// Change antch default dialer handler for proxy.
		antch.DefaultDialer = p.dial
	})

	if p.ProxyURL != nil {
		if proxyURL := p.ProxyURL(req); proxyURL != nil {
			req = req.WithContext(context.WithValue(req.Context(), ProxyKey{}, proxyURL))
		}
	}
	return p.Next.ProcessRequest(ctx, req)
}

type httpProxyDialer struct {
	host               string
	haveAuth           bool
	username, password string

	forward proxy.Dialer
}

func (p *httpProxyDialer) auth() string {
	if p.haveAuth {
		auth := p.username + ":" + p.password
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	}
	return ""
}

func (p *httpProxyDialer) Dial(network, addr string) (net.Conn, error) {
	conn, err := p.forward.Dial("tcp", p.host)
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
	if pa := p.auth(); pa != "" {
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

func httpProxy(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	h := &httpProxyDialer{
		host:    u.Host,
		forward: forward,
	}
	if u.User != nil {
		h.haveAuth = true
		h.username = u.User.Username()
		h.password, _ = u.User.Password()
	}
	return h, nil
}

func init() {
	// Registers proxy protocol type with associate with handler.
	proxy.RegisterDialerType("http", httpProxy)
	proxy.RegisterDialerType("https", httpProxy)
}
