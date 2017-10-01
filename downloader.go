package antch

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/antchfx/antch/internal/util"
)

var (
	// DefaultDialer is a default HTTP dial for HTTP connection
	// and is used by DownloaderStack.
	DefaultDialer DialFunc = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext
)

// Dialer specifies the dial function for creating HTTP connections.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Downloader is a web crawler handler to download web page from remote server
// for the given crawl request.
type Downloader interface {
	ProcessRequest(context.Context, *http.Request) (*http.Response, error)
}

// DownloaderFunc is an adapter type to allow the use of ordinary
// functions as Downloader handlers.
type DownloaderFunc func(context.Context, *http.Request) (*http.Response, error)

func (f DownloaderFunc) ProcessRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	return f(ctx, req)
}

// DownloaderMiddleware is middleware type for downloader.
type DownloaderMiddleware func(Downloader) Downloader

// httpClient returns Downloader that uses http.DefaultClient
// to execute HTTP request.
func httpClient() Downloader {
	return DownloaderFunc(func(_ context.Context, req *http.Request) (*http.Response, error) {
		return http.DefaultClient.Do(req)
	})
}

// DownloaderStack is an implementation of Downloader to allows register custom Downloader
// middleware to handle HTTP request and Http response.
type DownloaderStack struct {
	handler Downloader

	mids    []DownloaderMiddleware
	midOnce sync.Once

	ts     http.RoundTripper
	tsOnce sync.Once

	// Timeout specifies the maximum time to wait before
	// receive HTTP response.
	Timeout time.Duration
}

// UseMiddleware registeres a new middleware into Downloader.
func (dl *DownloaderStack) UseMiddleware(mid DownloaderMiddleware) *DownloaderStack {
	dl.mids = append(dl.mids, mid)
	return dl
}

// onceSetupMiddleware is setup all middlewares that registered by UseMiddleware function.
func (dl *DownloaderStack) onceSetupMiddleware() {
	var stack Downloader
	stack = DownloaderFunc(dl.send)
	for i := len(dl.mids) - 1; i >= 0; i-- {
		stack = dl.mids[i](stack)
	}
	dl.handler = stack
}

func (dl *DownloaderStack) deadline() time.Time {
	if dl.Timeout > 0 {
		return time.Now().Add(dl.Timeout)
	}
	return time.Time{}
}

// setRequestCancel sets the cancel HTTP request operation if deadline was reached.
func setRequestCancel(deadline time.Time, doCancel func()) (stopTimer func(), didTimeout func() bool) {
	var nop = func() {}
	var alwaysFalse = func() bool { return false }

	if deadline.IsZero() {
		return nop, alwaysFalse
	}

	stopTimerCh := make(chan struct{})
	var once sync.Once
	stopTimer = func() { once.Do(func() { close(stopTimerCh) }) }

	timer := time.NewTimer(time.Until(deadline))
	var timeout util.AtomicBool

	go func() {
		select {
		case <-timer.C:
			timeout.SetTrue()
			doCancel()
		case <-stopTimerCh:
			timer.Stop()
		}
	}()

	return stopTimer, timeout.IsSet
}

// send is sends a HTTP request and receive HTTP response from remote server.
func (dl *DownloaderStack) send(ctx context.Context, req *http.Request) (*http.Response, error) {
	deadline := dl.deadline()
	reqctx, doCancel := context.WithCancel(req.Context())
	stopTimer, didTimeout := setRequestCancel(deadline, doCancel)

	resc := make(chan responseAndError)
	go func() {
		resp, err := dl.transport().RoundTrip(req.WithContext(reqctx))
		resc <- responseAndError{res: resp, err: err}
	}()

	var re responseAndError
	select {
	case <-ctx.Done():
		doCancel()
		re = responseAndError{err: ctx.Err()}
	case re = <-resc:
	}

	if re.err != nil {
		stopTimer()
		err := re.err
		if !deadline.IsZero() && didTimeout() {
			err = &httpError{
				err:     err.Error() + " (spider timeout exceeded while awaiting headers)",
				timeout: true,
			}
		}
		return nil, err
	}

	if !deadline.IsZero() {
		re.res.Body = &cancelTimerBody{
			stop:          stopTimer,
			rc:            re.res.Body,
			reqDidTimeout: didTimeout,
		}
	}
	return re.res, nil
}

func (dl *DownloaderStack) initTransport() {
	dl.ts = &http.Transport{
		DialContext:           DefaultDialer,
		DisableCompression:    false,
		MaxIdleConns:          500,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func (dl *DownloaderStack) transport() http.RoundTripper {
	dl.tsOnce.Do(dl.initTransport)
	return dl.ts
}

func (dl *DownloaderStack) ProcessRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("antch: req is nil")
	}
	// Setup middlewares by once.
	dl.midOnce.Do(dl.onceSetupMiddleware)
	return dl.handler.ProcessRequest(ctx, req)
}

type httpError struct {
	err     string
	timeout bool
}

func (e *httpError) Error() string   { return e.err }
func (e *httpError) Timeout() bool   { return e.timeout }
func (e *httpError) Temporary() bool { return true }

// cancelTimerBody is an io.ReadCloser that wraps rc with two features:
// 1) on Read error or close, the stop func is called.
// 2) On Read failure, if reqDidTimeout is true, the error is wrapped and
//    marked as net.Error that hit its timeout.
// https://github.com/golang/go/blob/master/src/net/http/client.go#L800
type cancelTimerBody struct {
	stop          func()
	rc            io.ReadCloser
	reqDidTimeout func() bool
}

func (b *cancelTimerBody) Read(p []byte) (n int, err error) {
	n, err = b.rc.Read(p)
	if err == nil {
		return n, nil
	}
	b.stop()
	if err == io.EOF {
		return n, err
	}
	if b.reqDidTimeout() {
		err = &httpError{
			err:     err.Error() + " (spider timeout exceeded while reading body)",
			timeout: true,
		}
	}
	return n, err
}

func (b *cancelTimerBody) Close() error {
	err := b.rc.Close()
	b.stop()
	return err
}
