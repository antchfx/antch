package antch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/antchfx/antch/internal/util"
)

const DefaultMaxFetchersPerHost = 1

// Crawler is a web crawl server for crawl websites.
type Crawler struct {
	// MaxWorkers specifies the maximum number of worker to working.
	MaxWorkers int

	// MaxFetchersPerHost Specifies the number of fetcher
	// that should be allowed to access same host at one time.
	// If zero, DefaultMaxFetchersPerHost is used.
	MaxFetchersPerHost int

	// DownloadHandler specifies a download handler to fetching HTTP
	// response from remote server when server received a crawl request.
	// If No specifies Downloader, then default use http.DefaultClient to
	// execute HTTP request.
	DownloadHandler Downloader

	// MessageHandler specifies a message handler to handling received
	// HTTP response.
	// If No specifies Spider, then default use NoOpSpider to reponse all
	// received HTTP response.
	MessageHandler Spider

	isRunning int32
	poolSize  int

	fetcherMu sync.RWMutex
	fetchers  map[string]*fetcher

	exitCh    chan struct{}
	waitGroup util.WaitGroupWrapper
}

// DefaultCrawler is the default Crawler used to crawl website.
var DefaultCrawler = &Crawler{}

// Stop stops crawling and exit.
func (c *Crawler) Stop() error {
	if atomic.LoadInt32(&c.isRunning) == 0 {
		return errors.New("antch: crawler is not running")
	}
	close(c.exitCh)
	c.waitGroup.Wait()
	return nil
}

// Run starts server to begin crawling with URLs Queue.
func (c *Crawler) Run(q Queue) {
	if q == nil {
		panic("antch: nil queue")
	}
	c.exitCh = make(chan struct{})

	c.waitGroup.Wrap(func() { c.queueScanLoop(q) })

	atomic.StoreInt32(&c.isRunning, 1)
}

func (c *Crawler) downloader() Downloader {
	if c.DownloadHandler != nil {
		return c.DownloadHandler
	}
	return httpClient()
}

func (c *Crawler) spider() Spider {
	if c.MessageHandler != nil {
		return c.MessageHandler
	}
	return NoOpSpider()
}

func (c *Crawler) queueScanWorker(requestCh chan *http.Request, closeCh chan int) {
	newFetcher := func() *fetcher {
		return &fetcher{
			c:       c,
			queueCh: make(chan requestAndChan, c.maxFetchersPerHost()),
			quitCh:  make(chan struct{}),
		}
	}

	for {
		select {
		case req := <-requestCh:
			var (
				f     = c.getFetcher(req.URL.Host, newFetcher)
				resch = make(chan responseAndError)
				reqch = requestAndChan{ctx: context.Background(), req: req, ch: resch}
			)

			select {
			case f.queueCh <- reqch:
				// Waiting an HTTP response.
				select {
				case re := <-resch:
					if re.err != nil {
						logrus.Error(re.err)
					} else {
						c.spider().ProcessResponse(re.ctx, re.res)
						re.res.Body.Close()
					}
				case <-closeCh:
					return
				}
			case <-f.quitCh:
				// fetcher has exit work.
			case <-closeCh:
				return
			}
		case <-closeCh:
			return
		}
	}
}

func (c *Crawler) resizePool(requestCh chan *http.Request, closeCh chan int) {
	for {
		if c.poolSize == c.maxWorkers() {
			break
		} else if c.poolSize > c.maxWorkers() {
			// contract
			closeCh <- 1
			c.poolSize--
		} else {
			// expand
			c.waitGroup.Wrap(func() {
				c.queueScanWorker(requestCh, closeCh)
			})
			c.poolSize++
		}
	}
}

func (c *Crawler) queueScanLoop(q Queue) {
	requestCh := make(chan *http.Request, c.maxWorkers())
	closeCh := make(chan int, c.maxWorkers())

	refreshTicker := time.NewTicker(6 * time.Second)

	c.resizePool(requestCh, closeCh)

	for {
		select {
		case <-refreshTicker.C:
			c.resizePool(requestCh, closeCh)
		case <-c.exitCh:
			goto exit
		default:
			urlStr, err := q.Dequeue()
			switch {
			case err == io.EOF:
				// No URLs in the queue q.
				select {
				case <-time.After(200 * time.Millisecond):
				}
				continue
			case err != nil:
				// Got error.
				logrus.Error(err)
				continue
			}
			req, err := http.NewRequest("GET", urlStr, nil)
			if err != nil {
				continue
			}
			// Settings a User-Agent to identify by remote server.
			req.Header.Set("User-Agent", "antch")
			requestCh <- req
		}
	}

exit:
	close(closeCh)
	refreshTicker.Stop()
}

// getFetcher returns a fetcher to access for the specified website.
func (c *Crawler) getFetcher(host string, newf func() *fetcher) *fetcher {
	c.fetcherMu.RLock()
	f, ok := c.fetchers[host]
	c.fetcherMu.RUnlock()
	if ok {
		return f
	}

	c.fetcherMu.Lock()
	defer c.fetcherMu.Unlock()

	if c.fetchers == nil {
		c.fetchers = make(map[string]*fetcher)
	}
	f = newf()
	c.fetchers[host] = f

	go func() { f.fetchLoop() }()

	return f
}

func (c *Crawler) removeFetcher(f *fetcher) {
	c.fetcherMu.Lock()
	defer c.fetcherMu.Unlock()

	var host string
	for k, v := range c.fetchers {
		if v == f {
			host = k
			break
		}
	}
	delete(c.fetchers, host)
}

func (c *Crawler) maxWorkers() int {
	if v := c.MaxWorkers; v != 0 {
		return v
	}
	return runtime.NumCPU() * 3
}

func (c *Crawler) maxFetchersPerHost() int {
	if v := c.MaxFetchersPerHost; v != 0 {
		return v
	}
	return DefaultMaxFetchersPerHost
}

type requestAndChan struct {
	req *http.Request
	ctx context.Context
	ch  chan responseAndError
}

type responseAndError struct {
	res *http.Response
	ctx context.Context
	err error
}

// fetcher is a crawler worker to limit number of worker to fetch at one time
// for the same host.
type fetcher struct {
	c       *Crawler
	n       int                 // number of worker running
	queueCh chan requestAndChan // URLS queue with in same host.
	quitCh  chan struct{}       // worker exit channel.
}

func (f *fetcher) maxWorkers() int {
	return f.c.maxFetchersPerHost()
}

// fetchLoop runs in a single goroutine to execute download for a receved crawl request
// from its own URLs channel.
func (f *fetcher) fetchLoop() {
	workerPool := make(chan chan requestAndChan, f.maxWorkers())
	closeCh := make(chan int, f.maxWorkers())

	const idleTimeout = 10 * time.Minute
	idleTimer := time.NewTimer(idleTimeout)
	refreshTicker := time.NewTicker(8 * time.Second)

	f.resizePool(workerPool, closeCh)
	for {
		select {
		case reqCh := <-f.queueCh:
			select {
			case workCh := <-workerPool:
				select {
				case workCh <- reqCh:
					idleTimer.Reset(idleTimeout)
				default:
					// Workch has closed.
					d := reqCh
					go func() {
						f.queueCh <- d
					}()
				}
			case <-f.c.exitCh:
				// Main server has exit.
				goto exit
			}
		case <-refreshTicker.C:
			f.resizePool(workerPool, closeCh)
		case <-idleTimer.C:
			// Worker has inactive.
			f.c.removeFetcher(f)
			goto exit
		case <-f.c.exitCh:
			// Main server has exit.
			goto exit
		}
	}
exit:
	close(closeCh)
	close(f.quitCh)
	refreshTicker.Stop()
}

func (f *fetcher) resizePool(workerPool chan chan requestAndChan, closeCh chan int) {
	for {
		if f.n == f.maxWorkers() {
			break
		} else if f.n > f.maxWorkers() {
			// Decrease a worker number
			closeCh <- 1
			f.n--
		} else {
			// Increase a worker number
			go func() { f.fetchWorker(workerPool, closeCh) }()
			f.n++
		}
	}
}

func (f *fetcher) fetchWorker(workerPool chan chan requestAndChan, closeCh chan int) {
	workCh := make(chan requestAndChan)
	for {
		select {
		case workerPool <- workCh:
			select {
			case rc := <-workCh:
				resch := make(chan responseAndError)
				ctx, cancel := context.WithCancel(rc.ctx)

				go func() {
					defer closeRequest(rc.req)
					resp, err := f.c.downloader().ProcessRequest(ctx, rc.req)
					resch <- responseAndError{res: resp, err: err, ctx: rc.ctx}
				}()

				select {
				case re := <-resch:
					rc.ch <- re
				case <-closeCh:
					cancel()
					rc.ch <- responseAndError{err: errors.New("antch: request has canceled")}
					return
				}
			case <-closeCh:
				// exit a worker
				return
			}
		case <-closeCh:
			// exit a worker
			return
		}
	}
}

// Close HTTP request object.
func closeRequest(req *http.Request) {
	if req.Body != nil {
		req.Body.Close()
	}
}
