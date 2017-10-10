package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antchfx/antch"
)

var robotstxtTestBody = `User-agent: testbot
Disallow:

User-agent: *
Disallow: /`

func TestRobotstxtMiddlewareAllow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte(robotstxtTestBody))
			w.Header().Set("Content-Type", "text/plain")
			return
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ds := &antch.DownloaderStack{}
	fn := antch.DownloaderFunc(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return ds.ProcessRequest(ctx, req)
	})

	req, _ := http.NewRequest("GET", ts.URL, nil)

	mid := Robotstxt{Next: fn}

	mid.UserAgent = "testbot"
	_, err := mid.ProcessRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("expected nil error but got %v", err)
	}
}

func TestRobotstxtMiddlewareDisallow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte(robotstxtTestBody))
			w.Header().Set("Content-Type", "text/plain")
			return
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ds := &antch.DownloaderStack{}
	fn := antch.DownloaderFunc(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return ds.ProcessRequest(ctx, req)
	})

	req, _ := http.NewRequest("GET", ts.URL, nil)

	mid := Robotstxt{Next: fn}
	_, err := mid.ProcessRequest(context.Background(), req)
	if e, g := "robotstxt: request was denied", fmt.Sprintf("%v", err); e != g {
		t.Errorf("expected error %q, got %q", e, g)
	}
}
