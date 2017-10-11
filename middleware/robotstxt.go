package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/antchfx/antch"
	"github.com/temoto/robotstxt"
)

// Robotstxt is a middleware of Downloader for robots.txt.
type Robotstxt struct {
	// UserAgent specifies a user-agent value for robots.txt.
	// If No specifies value then take User-Agent of the request
	// header value as default UserAgent value.
	UserAgent string
	//
	Next antch.Downloader

	m  map[string]*robotsEntry
	mu sync.RWMutex
}

func (rt *Robotstxt) get(u *url.URL) *robotstxt.RobotsData {
	rt.mu.RLock()
	value, ok := rt.m[u.Host]
	rt.mu.RUnlock()
	if ok {
		value.update()
		return value.data
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.m == nil {
		rt.m = make(map[string]*robotsEntry)
	}
	addr := fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host)
	entry := &robotsEntry{
		url: addr,
	}
	entry.update()
	rt.m[u.Host] = entry
	return entry.data
}

func (rt *Robotstxt) useragent(req *http.Request) string {
	if v := rt.UserAgent; v != "" {
		return v
	}
	return req.Header.Get("User-Agent")
}

func (rt *Robotstxt) ProcessRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	robots := rt.get(req.URL)
	if !robots.TestAgent(req.URL.Path, rt.useragent(req)) {
		return nil, errors.New("robotstxt: request was denied")
	}
	return rt.Next.ProcessRequest(ctx, req)
}

type robotsEntry struct {
	data  *robotstxt.RobotsData
	url   string
	time  time.Time
	state int32
}

func (e *robotsEntry) update() {
	if time.Now().Sub(e.time).Hours() <= 24 {
		return
	}
	defer func() { e.time = time.Now() }()

	resp, err := http.Get(e.url)
	if err != nil {
		// If receive a robots.txt got error,
		// make all request to allowed.
		e.data = &robotstxt.RobotsData{}
		return
	}
	defer resp.Body.Close()
	data, err := robotstxt.FromResponse(resp)
	if err != nil {
		e.data = &robotstxt.RobotsData{}
		return
	}
	e.data = data
}
