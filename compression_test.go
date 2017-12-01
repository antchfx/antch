package antch

import (
	"compress/gzip"
	"compress/zlib"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompressionHandlerWithGzip(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		msg := []byte("hello world")
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		defer zw.Close()
		zw.Write(msg)
	}))
	defer ts.Close()
	testCompressionHandler(t, ts, []byte("hello world"))
}

func TestCompressionHandlerWithDeflate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		msg := []byte("hello world")
		w.Header().Set("Content-Encoding", "deflate")
		zw := zlib.NewWriter(w)
		defer zw.Close()
		zw.Write(msg)
	}))
	defer ts.Close()
	testCompressionHandler(t, ts, []byte("hello world"))
}

func testCompressionHandler(t *testing.T, ts *httptest.Server, want []byte) {
	handler := CompressionMiddleware()(defaultMessageHandler())
	req, _ := http.NewRequest("GET", ts.URL, nil)
	resp, err := handler.Send(req)
	if err != nil {
		t.Fatal(err)
	}
	if v := resp.Header.Get("Content-Encoding"); v != "" {
		t.Errorf("Content-Encoding = %s; want empty", v)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if v := string(b); v != string(want) {
		t.Errorf("body = %s; want %s", v, want)
	}
}
