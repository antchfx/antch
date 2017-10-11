package middleware

import (
	"compress/gzip"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antchfx/antch"
)

func TestHttpCompressionMiddleware(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		gw := gzip.NewWriter(w)
		gw.Write([]byte("hello world"))
		gw.Close()
	}))
	defer ts.Close()

	ds := &antch.DownloaderStack{}
	fn := antch.DownloaderFunc(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return ds.ProcessRequest(ctx, req)
	})

	req, _ := http.NewRequest("GET", ts.URL, nil)
	mid := &HttpCompression{Next: fn}

	resp, err := mid.ProcessRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	if resp.ContentLength != -1 {
		t.Fatalf("ContentLength = %d; want -1", resp.ContentLength)
	}

	if given, excepted := string(b), "hello world"; given != excepted {
		t.Fatalf("response body = %s; want %s", given, excepted)
	}
}
