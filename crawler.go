package antch

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Item is represents an item object.
type Item interface{}

// Crawler is core of web crawl server that provides crawl websites
// and calls pipeline to process for received data from their pages.
type Crawler struct {
	// CheckRedirect specifies the policy for handling redirects.
	CheckRedirect func(req *http.Request, via []*http.Request) error

	// MaxConcurrentRequests specifies the maximum number of concurrent
	// requests that will be performed.
	// Default is 16.
	MaxConcurrentRequests int

	// MaxConcurrentRequestsPerHost specifies the maximum number of
	// concurrent requests that will be performed to any single domain.
	// Default is 1.
	MaxConcurrentRequestsPerSite int

	// RequestTimeout specifies a time to wait before the HTTP Request times out.
	// Default is 30s.
	RequestTimeout time.Duration

	// DownloadDelay specifies delay time to wait before access same website.
	// Default is 0.25s.
	DownloadDelay time.Duration

	// MaxConcurrentItems specifies the maximum number of concurrent items
	// to process parallel in the pipeline.
	// Default is 32.
	MaxConcurrentItems int

	// UserAgent specifies the user-agent for the remote server.
	UserAgent string

	// ErrorLog specifies an optional logger for errors HTTP transports
	// and unexpected behavior from handlers.
	// If nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog Logger

	// Exit is an optional channel whose closure indicates that the Crawler
	// instance should be stop work and exit.
	Exit <-chan struct{}

	readCh  chan *http.Request
	writeCh chan Item

	client      *http.Client
	pipeHandler PipelineHandler
	mids        []Middleware
	pipes       []Pipeline

	spider   map[string]*spider
	spiderMu sync.Mutex

	once sync.Once
	mu   sync.RWMutex
	m    map[string]muxEntry
}

// NewCrawler returns a new Crawler with default settings.
func NewCrawler() *Crawler {
	return &Crawler{
		UserAgent: "antch(github.com)",
		Exit:      make(chan struct{}),
	}
}

type muxEntry struct {
	pattern string
	h       Handler
}

// StartURLs starts crawling for the given URL list.
func (c *Crawler) StartURLs(URLs []string) {
	for _, URL := range URLs {
		c.EnqueueURL(URL)
	}
}

// Crawl puts an HTTP request into the working queue to crawling.
func (c *Crawler) Crawl(req *http.Request) error {
	if req == nil {
		return errors.New("req is nil")
	}
	return c.enqueue(req, 5*time.Second)
}

// EnqueueURL puts given URL into the backup URLs queue.
func (c *Crawler) EnqueueURL(URL string) error {
	if URL == "" {
		return errors.New("URL is nil")
	}
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return err
	}
	return c.Crawl(req)
}

func (c *Crawler) enqueue(req *http.Request, timeout time.Duration) error {
	c.once.Do(c.init)
	select {
	case c.readCh <- req:
	case <-time.After(timeout):
		return errors.New("crawler: timeout, worker is busy")
	}
	return nil
}

// Handle registers the Handler for the given pattern.
// If pattern is "*" means will matches all requests if
// no any pattern matches.
func (c *Crawler) Handle(pattern string, handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if pattern == "" {
		panic("crawler: invalid domain")
	}
	if handler == nil {
		panic("crawler: handler is nil")
	}
	if c.m == nil {
		c.m = make(map[string]muxEntry)
	}
	c.m[pattern] = muxEntry{pattern: pattern, h: handler}
}

// Handler returns a Handler for the give HTTP Response.
func (c *Crawler) Handler(res *http.Response) (h Handler, pattern string) {
	return c.handler(res.Request.Host, res.Request.URL.Path)
}

// UseMiddleware adds a Middleware to the crawler.
func (c *Crawler) UseMiddleware(m ...Middleware) *Crawler {
	c.mids = append(c.mids, m...)
	return c
}

// UsePipeline adds a Pipeline to the crawler.
func (c *Crawler) UsePipeline(p ...Pipeline) *Crawler {
	c.pipes = append(c.pipes, p...)
	return c
}

// UseCookies enables the cookies middleware to working.
func (c *Crawler) UseCookies() *Crawler {
	return c.UseMiddleware(CookiesMiddleware())
}

// UseCompression enables the HTTP compression middleware to
// supports gzip, deflate for HTTP Request/Response.
func (c *Crawler) UseCompression() *Crawler {
	return c.UseMiddleware(CompressionMiddleware())
}

// UseProxy enables proxy for each of HTTP requests.
func (c *Crawler) UseProxy(proxyURL *url.URL) *Crawler {
	return c.UseMiddleware(ProxyMiddleware(http.ProxyURL(proxyURL)))
}

// UseRobotstxt enables support robots.txt.
func (c *Crawler) UseRobotstxt() *Crawler {
	return c.UseMiddleware(RobotstxtMiddleware())
}

func (c *Crawler) logf(format string, args ...interface{}) {
	if c.ErrorLog != nil {
		c.ErrorLog.Output(2, fmt.Sprintf(format, args...))
	} else {
		log.Printf(format, args...)
	}
}

func (c *Crawler) transport() http.RoundTripper {
	ts := &http.Transport{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   c.maxConcurrentRequestsPerSite() * 2,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext:           proxyDialContext,
	}

	var stack HttpMessageHandler = HttpMessageHandlerFunc(func(req *http.Request) (*http.Response, error) {
		return ts.RoundTrip(req)
	})
	for i := len(c.mids) - 1; i >= 0; i-- {
		stack = c.mids[i](stack)
	}

	return roundTripperFunc(stack.Send)
}

func (c *Crawler) pipeline() PipelineHandler {
	var stack PipelineHandler = PipelineHandlerFunc(func(item Item) {})
	for i := len(c.pipes) - 1; i >= 0; i-- {
		stack = c.pipes[i](stack)
	}
	return stack
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (c *Crawler) pathMatch(pattern, path string) bool {
	n := len(pattern)
	if pattern[n-1] == '/' {
		pattern = pattern[:n-1]
	}
	return strings.Index(path, pattern) >= 0
}

func (c *Crawler) matchHandler(path string) (h Handler, pattern string) {
	var n = 0
	for k, v := range c.m {
		if !c.pathMatch(k, path) {
			continue
		}
		if h == nil || len(k) > n {
			n = len(k)
			h = v.h
			pattern = v.pattern
		}
	}
	return
}

func (c *Crawler) handler(host, path string) (h Handler, pattern string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	h, pattern = c.matchHandler(host + path)
	if h == nil {
		h, pattern = c.matchHandler(host)
	}
	if h == nil {
		h, pattern = c.matchHandler("*")
	}
	if h == nil {
		h, pattern = VoidHandler(), ""
	}
	return
}

func (c *Crawler) maxConcurrentRequestsPerSite() int {
	if v := c.MaxConcurrentRequestsPerSite; v > 0 {
		return v
	}
	return 1
}

func (c *Crawler) maxConcurrentRequests() int {
	if v := c.MaxConcurrentRequests; v > 0 {
		return v
	}
	return 16
}

func (c *Crawler) maxConcurrentItems() int {
	if v := c.MaxConcurrentItems; v > 0 {
		return v
	}
	return 32
}

func (c *Crawler) downloadDelay() time.Duration {
	if v := c.DownloadDelay; v > 0 {
		return v
	}
	return 250 * time.Millisecond // 0.25s
}

func (c *Crawler) requestTimeout() time.Duration {
	if v := c.RequestTimeout; v > 0 {
		return v
	}
	return 30 * time.Second
}

func (c *Crawler) init() {
	c.client = &http.Client{
		Transport:     c.transport(),
		CheckRedirect: c.CheckRedirect,
		Timeout:       c.requestTimeout(),
	}

	c.pipeHandler = c.pipeline()
	c.readCh = make(chan *http.Request)
	c.writeCh = make(chan Item)
	go c.readLoop()
	go c.writeLoop()
}

func (c *Crawler) scanRequestWork(workCh chan chan *http.Request, closeCh chan int) {
	reqch := make(chan *http.Request)
	for {
		workCh <- reqch
		select {
		case req := <-reqch:
			resc := make(chan responseAndError)
			spider := c.getSpider(req.URL)

			if req.Header.Get("User-Agent") == "" && c.UserAgent != "" {
				req.Header.Set("User-Agent", c.UserAgent)
			}

			spider.reqch <- requestAndChan{req: req, ch: resc}
			select {
			case re := <-resc:
				closeRequest(req)
				if re.err != nil {
					c.logf("crawler: send HTTP request got error: %v", re.err)
				} else {
					go func(res *http.Response) {
						defer closeResponse(res)
						defer func() {
							if r := recover(); r != nil {
								c.logf("crawler: Handler got panic error: %v", r)
							}
						}()
						h, _ := c.Handler(res)
						h.ServeSpider(c.writeCh, res)
					}(re.res)
				}
			case <-closeCh:
				closeRequest(req)
				return
			}
		case <-closeCh:
			return
		}
	}
}

// readLoop reads HTTP crawl request from queue and to execute.
func (c *Crawler) readLoop() {
	closeCh := make(chan int)
	workCh := make(chan chan *http.Request, c.maxConcurrentRequests())

	for i := 0; i < c.maxConcurrentRequests(); i++ {
		go func() {
			c.scanRequestWork(workCh, closeCh)
		}()
	}

	for {
		select {
		case req := <-c.readCh:
			reqch := <-workCh
			reqch <- req
		case <-c.Exit:
			goto exit
		}
	}
exit:
	close(closeCh)
}

// writeLoop writes a received Item into the item pippeline.
func (c *Crawler) writeLoop() {
	closeCh := make(chan int)
	workCh := make(chan Item, c.maxConcurrentItems())

	for i := 0; i < c.maxConcurrentItems(); i++ {
		go func() {
			c.scanPipelineWork(workCh, closeCh)
		}()
	}
	for {
		select {
		case item := <-c.writeCh:
			workCh <- item
		case <-c.Exit:
			goto exit
		}
	}
exit:
	close(closeCh)
}

func (c *Crawler) scanPipelineWork(workCh chan Item, closeCh chan int) {
	for {
		select {
		case v := <-workCh:
			done := make(chan int)
			go func() {
				defer close(done)
				defer func() {
					if r := recover(); r != nil {
						c.logf("crawler: Handler got panic error: %v", r)
					}
				}()
				c.pipeHandler.ServePipeline(v)
			}()
			select {
			case <-done:
			case <-closeCh:
				return
			}
		case <-closeCh:
			return
		}
	}
}

// removeIdleSpider makes spider as dead.
func (c *Crawler) removeSpider(s *spider) {
	c.spiderMu.Lock()
	defer c.spiderMu.Unlock()
	delete(c.spider, s.key)
}

// getSpider returns a spider for the given URL.
func (c *Crawler) getSpider(url *url.URL) *spider {
	c.spiderMu.Lock()
	defer c.spiderMu.Unlock()

	if c.spider == nil {
		c.spider = make(map[string]*spider)
	}

	host, _, _ := net.SplitHostPort(url.Host)
	key := fmt.Sprintf("%s%s", url.Scheme, host)
	s, ok := c.spider[key]
	if !ok {
		s = &spider{
			c:           c,
			reqch:       make(chan requestAndChan),
			key:         key,
			idleTimeout: 120 * time.Second,
		}
		c.spider[key] = s
		go s.crawlLoop()
	}
	return s
}

type requestAndChan struct {
	req *http.Request
	ch  chan responseAndError
}

type responseAndError struct {
	res *http.Response
	err error
}

// spider is http spider for the single site.
type spider struct {
	c           *Crawler
	reqch       chan requestAndChan
	key         string
	idleTimeout time.Duration
}

func (s *spider) queueScanWorker(workCh chan chan requestAndChan, respCh chan int, closeCh chan struct{}) {
	rc := make(chan requestAndChan)
	for {
		workCh <- rc
		select {
		case c := <-rc:
			resp, err := s.c.client.Do(c.req)
			select {
			case c.ch <- responseAndError{resp, err}:
				respCh <- 1
			case <-closeCh:
				return
			}
		case <-closeCh:
			return
		}
	}
}

func (s *spider) crawlLoop() {
	respCh := make(chan int)
	closeCh := make(chan struct{})
	idleTimer := time.NewTimer(s.idleTimeout)
	workCh := make(chan chan requestAndChan, s.c.maxConcurrentRequestsPerSite())

	for i := 0; i < s.c.maxConcurrentRequestsPerSite(); i++ {
		go func() {
			s.queueScanWorker(workCh, respCh, closeCh)
		}()
	}

	for {
		select {
		case rc := <-s.reqch:
			// Wait a moment time before start fetching.
			if t := s.c.downloadDelay(); t > 0 {
				<-time.After(t)
			}
			c := <-workCh
			c <- rc
		case <-respCh:
			idleTimer.Reset(s.idleTimeout)
		case <-idleTimer.C:
			goto exit
		case <-s.c.Exit:
			goto exit
		}
	}

exit:
	s.c.removeSpider(s)
	close(closeCh)
	idleTimer.Stop()
}

func closeRequest(r *http.Request) {
	if r != nil && r.Body != nil {
		r.Body.Close()
	}
}

func closeResponse(r *http.Response) {
	if r != nil && r.Body != nil {
		r.Body.Close()
	}
}
