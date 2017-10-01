package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/antchfx/antch"
)

func TestDelayMiddleware(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ds := &antch.DownloaderStack{}
	fn := antch.DownloaderFunc(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return ds.ProcessRequest(ctx, req)
	})

	req, _ := http.NewRequest("GET", ts.URL, nil)

	mid := DownloadDelay{DelayTime: 100 * time.Millisecond, Next: fn}
	_, err := mid.ProcessRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-time.After(50 * time.Millisecond):
			cancel()
		}
	}()
	_, err = mid.ProcessRequest(ctx, req)
	if err != context.Canceled {
		t.Fatalf("expected error %q, got %q", err, context.Canceled)
	}
}
