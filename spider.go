package antch

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
)

// Spider is an interface for handle received HTTP response from
// remote server that HTTP request send by web crawler.
type Spider interface {
	ProcessResponse(context.Context, *http.Response) error
}

// SpiderFunc is an adapter type to allow the use of ordinary
// functions as Spider handlers.
type SpiderFunc func(context.Context, *http.Response) error

func (f SpiderFunc) ProcessResponse(ctx context.Context, res *http.Response) error {
	return f(ctx, res)
}

// NoOpSpider returns a Spider object that silently ignores all HTTP response
// without do anything.
func NoOpSpider() Spider {
	return SpiderFunc(func(_ context.Context, res *http.Response) error {
		// Make HTTP connection reusing for next HTTP request.
		// (https://stackoverflow.com/questions/17948827/reusing-http-connections-in-golang)
		io.Copy(ioutil.Discard, res.Body)
		return nil
	})
}

// SpiderMux is a multiplexer for handle HTTP response.
// It matches a registered handler based on host name of
// HTTP response URL to handle.
type SpiderMux struct {
	mu sync.Mutex
	m  map[string]spiderMuxEntry
}

type spiderMuxEntry struct {
	pattern  string
	explicit bool
	h        Spider
}

func (mux *SpiderMux) match(host string) (h Spider, pattern string) {
	if entry, ok := mux.m[host]; ok {
		h, pattern = entry.h, entry.pattern
	}
	return
}

func (mux *SpiderMux) handler(host string) (h Spider, pattern string) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	h, pattern = mux.match(host)
	if h == nil {
		h, pattern = NoOpSpider(), ""
	}
	return
}

// Handler returns the handler to use for the given host name.
func (mux *SpiderMux) Handler(host string) (h Spider, pattern string) {
	return mux.handler(host)
}

// Handle registers the handler for the given host name.
func (mux *SpiderMux) Handle(pattern string, handler Spider) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	if pattern == "" {
		panic("antch: invalid host")
	}
	if handler == nil {
		panic("antch: handler is nil")
	}
	if mux.m[pattern].explicit == true {
		panic("antch: multiple registrations for " + pattern)
	}

	if mux.m == nil {
		mux.m = make(map[string]spiderMuxEntry)
	}
	mux.m[pattern] = spiderMuxEntry{explicit: true, pattern: pattern, h: handler}
}

// HandleFunc registers the handler function for the given host name.
func (mux *SpiderMux) HandleFunc(host string, handler func(context.Context, *http.Response) error) {
	mux.Handle(host, SpiderFunc(handler))
}

func (mux *SpiderMux) ProcessResponse(ctx context.Context, res *http.Response) error {
	h, _ := mux.Handler(res.Request.URL.Host)
	return h.ProcessResponse(ctx, res)
}
