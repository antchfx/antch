package antch

import (
	"net/http"
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"
)

func cookiesHandler(next HttpMessageHandler) HttpMessageHandler {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return HttpMessageHandlerFunc(func(req *http.Request) (*http.Response, error) {
		// Delete previous cookie value before set new cookie value.
		req.Header.Del("Cookie")

		for _, cookie := range jar.Cookies(req.URL) {
			req.AddCookie(cookie)
		}

		resp, err := next.Send(req)
		if err != nil {
			return nil, err
		}
		if rc := resp.Cookies(); len(rc) > 0 {
			jar.SetCookies(req.URL, rc)
		}
		return resp, err
	})
}

// CookiesMiddleware is an HTTP cookies middleware to allows cookies
// to tracking for each of HTTP requests.
func CookiesMiddleware() Middleware {
	return Middleware(cookiesHandler)
}
