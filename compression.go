package antch

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
)

func decompress(name string, rc io.ReadCloser) (io.ReadCloser, bool) {
	switch name {
	case "gzip":
		return &gzipReader{rc: rc}, true
	case "deflate":
		return &deflateReader{rc: rc}, true
	}
	return nil, false
}

func compressionHandler(next HttpMessageHandler) HttpMessageHandler {
	return HttpMessageHandlerFunc(func(req *http.Request) (*http.Response, error) {
		req.Header.Set("Accept-Encoding", "gzip, deflate")

		resp, err := next.Send(req)
		if err != nil {
			return nil, err
		}
		if rc, ok := decompress(resp.Header.Get("Content-Encoding"), resp.Body); ok {
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")

			resp.Body = rc
			resp.ContentLength = -1
			resp.Uncompressed = true
		}
		return resp, err
	})
}

// gzipReader is a reader with gzip decompress mode.
type gzipReader struct {
	rr io.Reader
	rc io.ReadCloser
}

func (z *gzipReader) Read(p []byte) (n int, err error) {
	if z.rr == nil {
		z.rr, err = gzip.NewReader(z.rc)
		if err != nil {
			return n, err
		}
	}
	return z.rr.Read(p)
}

func (z *gzipReader) Close() error {
	return z.rc.Close()
}

// deflateReader is a reader with deflate decompress mode.
type deflateReader struct {
	rr io.Reader
	rc io.ReadCloser
}

func (r *deflateReader) Read(p []byte) (n int, err error) {
	if r.rr == nil {
		r.rr, err = zlib.NewReader(r.rc)
		if err != nil {
			return n, err
		}
	}
	return r.rr.Read(p)
}

func (r *deflateReader) Close() error {
	return r.rc.Close()
}

// CompressionMiddleware is a middleware to allows compressed
// (gzip, deflate) traffic to be sent/received from sites.
func CompressionMiddleware() Middleware {
	return Middleware(compressionHandler)
}
