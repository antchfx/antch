package middleware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/antchfx/antch"
)

// Redirect is a middleware of Downloader for handling redirects
// from server.
type Redirect struct {
	// CheckRedirect specifies the policy for handling redirects.
	CheckRedirect func(req *http.Request, via []*http.Request) error

	//
	Next antch.Downloader
}

func defaultCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

func (r *Redirect) checkRedirect(req *http.Request, via []*http.Request) error {
	fn := r.CheckRedirect
	if fn == nil {
		fn = defaultCheckRedirect
	}
	return fn(req, via)
}

func (rd *Redirect) ProcessRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	var (
		reqs        []*http.Request
		resp        *http.Response
		copyHeaders = makeHeadersCopier(req)

		redirectMethod string
		includeBody    bool
	)

	uerr := func(err error) error {
		method := reqs[0].Method
		var urlStr string
		if resp != nil && resp.Request != nil {
			urlStr = resp.Request.URL.String()
		} else {
			urlStr = req.URL.String()
		}
		return &url.Error{
			Op:  method[:1] + strings.ToLower(method[1:]),
			URL: urlStr,
			Err: err,
		}
	}

	for {
		if len(reqs) > 0 {
			loc := resp.Header.Get("Location")
			if loc == "" {
				return nil, uerr(fmt.Errorf("redirct: %d response missing Location header", resp.StatusCode))
			}
			u, err := req.URL.Parse(loc)
			if err != nil {
				return nil, uerr(fmt.Errorf("redirct: failed to parse Location header %q: %v", loc, err))
			}

			ireq := reqs[0]
			req = &http.Request{
				Method:   redirectMethod,
				Response: resp,
				URL:      u,
				Header:   make(http.Header),
				Cancel:   ireq.Cancel,
			}
			req = req.WithContext(ireq.Context())

			if includeBody && ireq.GetBody != nil {
				req.Body, err = ireq.GetBody()
				if err != nil {
					return nil, uerr(err)
				}
				req.ContentLength = ireq.ContentLength
			}

			copyHeaders(req)

			if ref := refererForURL(reqs[len(reqs)-1].URL, req.URL); ref != "" {
				req.Header.Set("Referer", ref)
			}

			err = rd.checkRedirect(req, reqs)
			if err == http.ErrUseLastResponse {
				return resp, nil
			}

			const maxBodySlurpSize = 2 << 10
			if resp.ContentLength == -1 || resp.ContentLength <= maxBodySlurpSize {
				io.CopyN(ioutil.Discard, resp.Body, maxBodySlurpSize)
			}
			resp.Body.Close()

			if err != nil {
				ue := uerr(err)
				ue.(*url.Error).URL = loc
				return resp, ue
			}
		}

		reqs = append(reqs, req)
		var err error
		if resp, err = rd.Next.ProcessRequest(ctx, req); err != nil {
			return nil, uerr(err)
		}

		var shouldRedirect bool
		redirectMethod, shouldRedirect, includeBody = redirectBehavior(req.Method, resp, reqs[0])
		if !shouldRedirect {
			return resp, nil
		}
	}
}

// makeHeadersCopier makes a function that copies headers from the
// initial Request, ireq.
func makeHeadersCopier(ireq *http.Request) func(*http.Request) {
	var (
		ireqhdr  = cloneHeader(ireq.Header)
		icookies map[string][]*http.Cookie
	)
	if ireq.Header.Get("Cookie") != "" {
		icookies = make(map[string][]*http.Cookie)
		for _, c := range ireq.Cookies() {
			icookies[c.Name] = append(icookies[c.Name], c)
		}
	}
	preq := ireq
	return func(req *http.Request) {
		if icookies != nil {
			var changed bool
			resp := req.Response // The response that caused the upcoming redirect
			for _, c := range resp.Cookies() {
				if _, ok := icookies[c.Name]; ok {
					delete(icookies, c.Name)
					changed = true
				}
			}
			if changed {
				ireqhdr.Del("Cookie")
				var ss []string
				for _, cs := range icookies {
					for _, c := range cs {
						ss = append(ss, c.Name+"="+c.Value)
					}
				}
				sort.Strings(ss)
				ireqhdr.Set("Cookie", strings.Join(ss, "; "))
			}
		}
		// Copy the initial request's Header values
		// (at least the safe ones).
		for k, vv := range ireqhdr {
			if shouldCopyHeaderOnRedirect(k, preq.URL, req.URL) {
				req.Header[k] = vv
			}
		}

		preq = req // Update previous Request with the current request
	}
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

// redirectBehavior describes what should happen when the
// client encounters a 3xx status code from the server
func redirectBehavior(reqMethod string, resp *http.Response, ireq *http.Request) (redirectMethod string, shouldRedirect, includeBody bool) {
	switch resp.StatusCode {
	case 301, 302, 303:
		redirectMethod = reqMethod
		shouldRedirect = true
		includeBody = false

		if reqMethod != "GET" && reqMethod != "HEAD" {
			redirectMethod = "GET"
		}
	case 307, 308:
		redirectMethod = reqMethod
		shouldRedirect = true
		includeBody = true

		if resp.Header.Get("Location") == "" {
			shouldRedirect = false
			break
		}
		if ireq.GetBody == nil && ireq.Body != nil && ireq.Body != http.NoBody {
			shouldRedirect = false
		}
	}
	return redirectMethod, shouldRedirect, includeBody
}

func refererForURL(lastReq, newReq *url.URL) string {
	if lastReq.Scheme == "https" && newReq.Scheme == "http" {
		return ""
	}
	referer := lastReq.String()
	if lastReq.User != nil {
		auth := lastReq.User.String() + "@"
		referer = strings.Replace(referer, auth, "", 1)
	}
	return referer
}

func shouldCopyHeaderOnRedirect(headerKey string, initial, dest *url.URL) bool {
	switch http.CanonicalHeaderKey(headerKey) {
	case "Authorization", "Www-Authenticate", "Cookie", "Cookie2":
		ihost := strings.ToLower(initial.Host)
		dhost := strings.ToLower(dest.Host)
		return isDomainOrSubdomain(dhost, ihost)
	}
	// All other headers are copied:
	return true
}

func isDomainOrSubdomain(sub, parent string) bool {
	if sub == parent {
		return true
	}
	// If sub is "foo.example.com" and parent is "example.com",
	// that means sub must end in "."+parent.
	// Do it without allocating.
	if !strings.HasSuffix(sub, parent) {
		return false
	}
	return sub[len(sub)-len(parent)-1] == '.'
}
