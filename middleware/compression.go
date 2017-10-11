package middleware

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"io"
	"net/http"

	"github.com/antchfx/antch"
)

// HttpCompression is a middleware of Downloader to allows compressed
// (gzip, deflate) traffic to be sent/received from web sites.
type HttpCompression struct {
	Next antch.Downloader
}

func (c *HttpCompression) ProcessRequest(ctx context.Context, req *http.Request) (resp *http.Response, err error) {
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	resp, err = c.Next.ProcessRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if body, ok := decompress(resp.Header.Get("Content-Encoding"), resp.Body); ok {
		resp.Body = body
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		resp.ContentLength = -1
		resp.Uncompressed = true
	}
	return
}

func decompress(name string, body io.ReadCloser) (io.ReadCloser, bool) {
	switch name {
	case "gzip":
		return &gzipReader{body: body}, true
	case "deflate":
		return &deflateReader{body: body}, true
	}
	return nil, false
}

// gzipReader is a reader with gzip decompress mode.
type gzipReader struct {
	r    io.Reader
	body io.ReadCloser
}

func (z *gzipReader) Read(p []byte) (int, error) {
	if z.r == nil {
		var err error
		z.r, err = gzip.NewReader(z.body)
		if err != nil {
			return 0, err
		}
	}
	return z.r.Read(p)
}

func (z *gzipReader) Close() error {
	return z.body.Close()
}

// deflateReader is a reader with deflate decompress mode.
type deflateReader struct {
	r    io.Reader
	body io.ReadCloser
}

func (r *deflateReader) Read(p []byte) (int, error) {
	if r.r == nil {
		rc, err := zlib.NewReader(r.body)
		if err != nil {
			return 0, err
		}
		r.r = rc
	}
	return r.r.Read(p)
}

func (r *deflateReader) Close() error {
	return r.body.Close()
}
