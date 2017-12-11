package antch

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

const robotsText = "User-agent: * \nDisallow: /account/\nDisallow: /ping\nAllow: /shopping/$\nUser-agent: Twitterbot\nDisallow: /\nSitemap: http://www.bing.com/dict/sitemap-index.xml"

func TestRobotstxtHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(robotsText))
		case "/":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	pathTests := []struct {
		path    string
		ua      string
		allowed bool
	}{
		{"/", "", true},
		{"/shopping/", "", true},
		{"account", "", false},
		{"/", "Twitterbot", false},
	}

	handler := RobotstxtMiddleware()(defaultMessageHandler())
	for _, test := range pathTests {
		req, err := http.NewRequest("GET", ts.URL+test.path, nil)
		if err != nil {
			t.Fatalf("NewRequest failed: %v", err)
		}
		if ua := test.ua; ua != "" {
			req.Header.Set("User-Agent", ua)
		}

		_, err = handler.Send(req)
		if test.allowed && err != nil {
			t.Errorf("%s(%s) err = %v; want nil", test.path, test.ua, err)
		} else if !test.allowed && err == nil {
			t.Errorf("%s(%s) err = %v; want request was deny error", test.path, test.ua, err)
		}
	}

}

func TestRobotstxtWithProxyHandler(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		director := func(req *http.Request) {
			req.URL.Host = r.Host
			req.URL.Scheme = "http"
		}
		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(w, r)
	}))
	defer proxyServer.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(robotsText))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)
	handler := ProxyMiddleware(http.ProxyURL(proxyURL))(RobotstxtMiddleware()(defaultMessageHandler()))

	req, _ := http.NewRequest("GET", ts.URL, nil)
	_, err := handler.Send(req)
	if err != nil {
		t.Errorf("request path /, err = %v; want nil", err)
	}
}
