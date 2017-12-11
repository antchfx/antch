package antch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

type robotsEntry struct {
	data *robotstxt.RobotsData
	url  string
	last time.Time
}

func (e *robotsEntry) update(proxyURL *url.URL) {
	e.last = time.Now()
	allAllowed := func() *robotstxt.RobotsData {
		return &robotstxt.RobotsData{}
	}
	ts := &http.Transport{
		DialContext: proxyDialContext,
	}
	client := &http.Client{
		Transport: ts,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, _ := http.NewRequest("GET", e.url, nil)
	if proxyURL != nil {
		req = req.WithContext(context.WithValue(req.Context(), ProxyKey{}, proxyURL))
	}
	resp, err := client.Do(req)
	if err != nil {
		e.data = allAllowed()
		return
	}
	defer resp.Body.Close()
	data, err := robotstxt.FromResponse(resp)
	if err == nil {
		e.data = data
	} else {
		e.data = allAllowed()
	}
}

func robotstxtHandler(next HttpMessageHandler) HttpMessageHandler {
	var (
		mu sync.RWMutex
		m  = make(map[string]*robotsEntry)
	)

	get := func(URL string, proxyURL *url.URL) *robotstxt.RobotsData {
		mu.RLock()
		e := m[URL]
		mu.RUnlock()

		if e == nil {
			mu.Lock()
			defer mu.Unlock()
			e = &robotsEntry{url: URL}
			e.update(proxyURL)
			m[URL] = e
			return e.data
		}

		if (time.Now().Sub(e.last).Hours()) >= 24 {
			go e.update(proxyURL)
		}
		return e.data
	}

	return HttpMessageHandlerFunc(func(req *http.Request) (*http.Response, error) {
		var proxyURL *url.URL
		if v := req.Context().Value(ProxyKey{}); v != nil {
			proxyURL = v.(*url.URL)
		}
		r := get(robotstxtURL(req.URL), proxyURL)
		ua := req.Header.Get("User-Agent")
		if r.TestAgent(req.URL.Path, ua) {
			return next.Send(req)
		}
		return nil, errors.New("request was denied by robots.txt")
	})
}

func robotstxtURL(u *url.URL) string {
	return fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host)
}

// RobotstxtMiddleware is a middleware for robots.txt, make HTTP
// request is more polite.
func RobotstxtMiddleware() Middleware {
	return Middleware(robotstxtHandler)
}
