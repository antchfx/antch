package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antchfx/antch"
)

func TestRedirectMiddleware(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}))
	defer ts.Close()

	tr := &http.Transport{}
	defer tr.CloseIdleConnections()

	ds := &antch.DownloaderStack{}
	fn := antch.DownloaderFunc(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return ds.ProcessRequest(ctx, req)
	})

	req, _ := http.NewRequest("GET", ts.URL, nil)

	mid := Redirect{Next: fn}
	_, err := mid.ProcessRequest(context.Background(), req)
	if e, g := "Get /: stopped after 10 redirects", fmt.Sprintf("%v", err); e != g {
		t.Errorf("expected error %q, got %q", e, g)
	}
}
