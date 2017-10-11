package middleware

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/antchfx/antch"
	"github.com/tylertreat/BoomFilters"
)

// UrlDupeFilter is a middleware of Downloader is simple to filter
// duplicate URLs that had already been seen before start request.
type UrlDupeFilter struct {
	Next antch.Downloader

	mu   sync.RWMutex
	once sync.Once
	boom boom.Filter
}

type UrlDupeIgnoredKey struct{}

func (f *UrlDupeFilter) ProcessRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	f.once.Do(func() {
		f.boom = boom.NewDefaultScalableBloomFilter(0.01)
	})

	// If request context has `UrlDupeIgnoredKey` means current request
	// ignored to process by this middleware.
	if v := ctx.Value(UrlDupeIgnoredKey{}); v != nil {
		return f.Next.ProcessRequest(ctx, req)
	}
	key := []byte(req.URL.String())

	f.mu.RLock()
	seen := f.boom.Test(key)
	f.mu.RUnlock()

	if seen {
		return nil, errors.New("urldupefilter: request was denied")
	}

	res, err := f.Next.ProcessRequest(ctx, req)
	if err == nil {
		f.mu.Lock()
		f.boom.Add(key)
		f.mu.Unlock()
	}
	return res, err
}
