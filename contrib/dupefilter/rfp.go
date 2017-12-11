package dupefilter

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/antchfx/antch"
	"github.com/tylertreat/BoomFilters"
)

// RFPDupeFilter is a middleware of HTTP downloader to filter duplicate
// requests, based on request fingerprint.
type RFPDupeFilter struct {
	// IncludeHeaders specifies what header value is included to
	// fingerprint compute.
	IncludeHeaders []string

	mu   sync.Mutex
	boom boom.Filter
	next antch.HttpMessageHandler
}

func canonicalizeURL(u *url.URL) (n *url.URL) {
	n = &url.URL{
		Scheme:  u.Scheme,
		User:    u.User,
		Host:    u.Host,
		Path:    u.Path,
		RawPath: u.RawPath,
	}

	if host, port, _ := net.SplitHostPort(u.Host); (u.Scheme == "http" && port == "80") ||
		(u.Scheme == "https" && port == "443") {
		n.Host = host
	}
	if v := u.EscapedPath(); v == "" {
		n.Path = "/"
		n.RawPath = "/"
	}
	// The query params.
	var querys []string
	for name, value := range u.Query() {
		for _, v := range value {
			querys = append(querys, name+"="+url.QueryEscape(v))
		}
	}
	// Sorted query list by it's name.
	a := sort.StringSlice(querys)
	sort.Sort(a)
	if len(a) > 0 {
		n.RawQuery = strings.Join(a, "&")
	}
	return n
}

func fingerprint(req *http.Request, includeHeaders []string) []byte {
	var fp bytes.Buffer
	method := req.Method
	if req.Method == "" {
		method = "GET"
	}
	fp.WriteString(method)
	fp.WriteString(canonicalizeURL(req.URL).String())
	switch req.Method {
	case "POST", "DELETE", "PUT":
		if r, err := req.GetBody(); err == nil && r != http.NoBody {
			if b, err := ioutil.ReadAll(r); err == nil {
				h := md5.New()
				fp.WriteString(fmt.Sprintf("%x", h.Sum(b)))
			}
		}
	}
	if includeHeaders != nil {
		for name, value := range req.Header {
			for _, name2 := range includeHeaders {
				if name == name2 {
					fp.WriteString(strings.Join(value, "&"))
				}
			}
		}
	}
	return fp.Bytes()
}

func (f *RFPDupeFilter) Send(req *http.Request) (*http.Response, error) {
	// A request is specifies force to crawling.
	if v, ok := req.Context().Value("dont_filter").(bool); ok && v {
		return f.next.Send(req)
	}
	fp := fingerprint(req, f.IncludeHeaders)
	f.mu.Lock()
	if f.boom.TestAndAdd(fp) {
		f.mu.Unlock()
		// Is has visited before.
		return nil, errors.New("RFPDupeFilter: request was denied")
	}
	f.mu.Unlock()
	return f.next.Send(req)
}

func RFPDupeFilterMiddleware() antch.Middleware {
	return func(next antch.HttpMessageHandler) antch.HttpMessageHandler {
		bf := boom.NewDefaultScalableBloomFilter(0.01)
		return &RFPDupeFilter{next: next, boom: bf}
	}
}
