package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/antchfx/antch"
)

// DownloadDelay is a middleware of Downloader to delay to download
// for each of HTTP request.
type DownloadDelay struct {
	// DelayTime specifies delay time to wait before access website.
	// If Zero, then default delay time(200ms) is used.
	DelayTime time.Duration

	Next antch.Downloader
}

//
type DownloadDelayKey struct{}

const DefaultDelayTime = 200 * time.Millisecond

func (d *DownloadDelay) ProcessRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	dt := d.DelayTime
	// If request context have a crawl-delay value,
	// then this delay is used instead.
	if v := req.Context().Value(DownloadDelayKey{}); v != nil {
		dt = v.(time.Duration)
	}

	if dt > 0 {
		select {
		case <-ctx.Done():
			return nil, context.Canceled
		case <-time.After(dt):
		}
	}

	return d.Next.ProcessRequest(ctx, req)
}
