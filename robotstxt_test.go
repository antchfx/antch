package antch

import (
	"net/http"
	"net/http/httptest"
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
