package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyMiddleware(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	}))
	defer ts.Close()
}
