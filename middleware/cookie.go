package middleware

import (
	"context"
	"net/http"
	"net/http/cookiejar"

	"github.com/antchfx/antch"
	"golang.org/x/net/publicsuffix"
)

var defaultCookieJar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

// Cookie is a middleware of Downloader for HTTP cookies
// management for each of HTTP request and HTTP response.
type Cookie struct {
	// Jar specifies the cookie jar.
	Jar http.CookieJar

	Next antch.Downloader
}

func (c *Cookie) jar() http.CookieJar {
	if c.Jar != nil {
		return c.Jar
	}
	return defaultCookieJar
}

func (c *Cookie) ProcessRequest(ctx context.Context, req *http.Request) (resp *http.Response, err error) {
	// Delete previous cookie value before set new cookie value.
	req.Header.Del("Cookie")

	jar := c.jar()

	for _, cookie := range jar.Cookies(req.URL) {
		req.AddCookie(cookie)
	}

	if resp, err = c.Next.ProcessRequest(ctx, req); err == nil {
		if rc := resp.Cookies(); len(rc) > 0 {
			jar.SetCookies(req.URL, rc)
		}
	}
	return resp, err
}
