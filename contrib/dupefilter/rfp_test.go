package dupefilter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/antchfx/antch"
)

func TestCanonicalizeURL(t *testing.T) {
	testURLs := []struct {
		scheme   string
		user     string
		host     string
		path     string
		querys   map[string]string
		fragment string
		wantURL  string
	}{
		{"http", "", "example.com", "", nil, "", "http://example.com/"},
		{"http", "test:test", "@www.example.com:80", "/auth", nil, "", "http://test:test@www.example.com/auth"},
		{"http", "", "example.com", "/", map[string]string{
			"q": "go%2Blanguage",
		}, "#1", "http://example.com/?q=go%2Blanguage"},
		{"https", "", "example.com:443", "", nil, "#ref=ref", "https://example.com/"},
	}

	for _, testURL := range testURLs {
		var urlstr string
		urlstr = fmt.Sprintf("%s://%s%s%s", testURL.scheme, testURL.user, testURL.host, testURL.path)
		if len(testURL.querys) > 0 {
			urlstr = urlstr + "?"
			var q []string
			for name, value := range testURL.querys {
				q = append(q, name+"="+value)
			}
			urlstr = urlstr + strings.Join(q, "&")
		}
		urlstr = urlstr + testURL.fragment
		u, err := url.Parse(urlstr)
		if err != nil {
			t.Fatal(err)
		}
		u2 := canonicalizeURL(u)
		if g, e := u2.String(), testURL.wantURL; g != e {
			t.Errorf("expected %s; but got %s", e, g)
		}
	}
}

func TestFingerprint(t *testing.T) {
	testFingerprints := []struct {
		req  *http.Request
		want []byte
	}{
		{&http.Request{
			Method: "GET",
			Header: map[string][]string{
				"ETag": []string{"33a64df551425fcc55e4d42a148795d9f25f89d4"},
			},
			URL: &url.URL{
				Scheme: "http",
				Host:   "example.com",
				Path:   "/",
			}},
			[]byte("GEThttp://example.com/33a64df551425fcc55e4d42a148795d9f25f89d4"),
		},
		{
			&http.Request{
				Method: "POST",
				URL: &url.URL{
					Scheme: "http",
					Host:   "example.com",
					Path:   "/login",
				},
				GetBody: func() (io.ReadCloser, error) {
					buf := []byte("hello,world")
					r := bytes.NewReader(buf)
					return ioutil.NopCloser(r), nil
				}},
			[]byte("POSThttp://example.com/login68656c6c6f2c776f726c64d41d8cd98f00b204e9800998ecf8427e"),
		},
	}
	includeHeaders := []string{"ETag"}
	for _, testfp := range testFingerprints {
		if g, e := string(fingerprint(testfp.req, includeHeaders)), string(testfp.want); e != g {
			t.Errorf("expected %s; but got %s", e, g)
		}
	}
}

func TestRFPHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	handler := RFPDupeFilterMiddleware()(antch.HttpMessageHandlerFunc(func(req *http.Request) (*http.Response, error) {
		return http.DefaultClient.Do(req)
	}))
	// First Request.
	req, _ := http.NewRequest("GET", ts.URL+"/?q=go", nil)
	resp, err := handler.Send(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected HTTP Status-Code is 200, but got %d", resp.StatusCode)
	}

	// Send Request will be deny by handler.
	_, err = handler.Send(req)
	if err == nil {
		t.Fatalf("want nil, but got %v", err)
	}
	if g, e := err.Error(), "RFPDupeFilter: request was denied"; g != e {
		t.Fatalf("expected %s; but got %s", e, g)
	}

	req = req.WithContext(context.WithValue(req.Context(), "dont_filter", true))
	resp, _ = handler.Send(req)
	if resp.StatusCode != 200 {
		t.Fatalf("expected HTTP Status-Code is 200, but got %d", resp.StatusCode)
	}
}
