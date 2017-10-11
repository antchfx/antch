package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antchfx/antch"
)

func TestUrlDupeFilterMiddleware(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ds := &antch.DownloaderStack{}
	fn := antch.DownloaderFunc(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return ds.ProcessRequest(ctx, req)
	})

	req, _ := http.NewRequest("GET", ts.URL, nil)
	mid := &UrlDupeFilter{Next: fn}

	_, err := mid.ProcessRequest(context.Background(), req)
	// first request.
	if err != nil {
		t.Fatal(err)
	}

	// second request.
	_, err = mid.ProcessRequest(context.Background(), req)
	if e, g := "urldupefilter: request was denied", fmt.Sprintf("%v", err); e != g {
		t.Errorf("expected error %q, got %q", e, g)
	}
}
