package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/antchfx/antch"
)

func TestCookieMiddleware(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, cookie := range r.Cookies() {
			http.SetCookie(w, cookie)
		}
	}))
	defer ts.Close()

	var expectedCookies = []*http.Cookie{
		{Name: "ChocolateChip", Value: "tasty"},
		{Name: "First", Value: "Hit"},
		{Name: "Second", Value: "Hit"},
	}

	ds := antch.DownloaderStack{}
	fn := antch.DownloaderFunc(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return ds.ProcessRequest(ctx, req)
	})

	u, _ := url.Parse(ts.URL)
	mid := &Cookie{Jar: defaultCookieJar, Next: fn}
	mid.Jar.SetCookies(u, expectedCookies)

	req, _ := http.NewRequest("GET", ts.URL, nil)
	resp, err := mid.ProcessRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	givenCookies := resp.Cookies()
	if len(givenCookies) != len(expectedCookies) {
		t.Errorf("Expected %d cookies, got %d", len(expectedCookies), len(givenCookies))
	}

	for _, ec := range expectedCookies {
		foundC := false
		for _, c := range givenCookies {
			if ec.Name == c.Name && ec.Value == c.Value {
				foundC = true
				break
			}
		}
		if !foundC {
			t.Errorf("Missing cookie %v", ec)
		}
	}
}
