package antch

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestCrawlerBasic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	tc := NewCrawler()
	// The custom spider handler.
	tc.Handle("*", HandlerFunc(func(c chan<- Item, resp *http.Response) {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}
		c <- string(b)
	}))

	c := make(chan Item)
	// The custom pipeline handler.
	tc.UsePipeline(func(_ PipelineHandler) PipelineHandler {
		return PipelineHandlerFunc(func(v Item) {
			c <- v
		})
	})

	tc.StartURLs([]string{ts.URL})
	// Waiting to receive a value from crawler tc.
	if g, e := (<-c).(string), "ok"; g != e {
		t.Errorf("expected %s; got %s", e, g)
	}
}

func TestCrawlerSpiderMux(t *testing.T) {
	var serveFakes = []struct {
		host string
		path string
		code int
	}{
		{"example.com", "/", 200},
		{"example.com", "/search", 201},
		{"localhost", "/", 200},
	}

	var spiderMuxTests = []struct {
		pattern string
		code    int
	}{
		{"example.com", 200},
		{"example.com/search", 201},
		{"localhost", 200},
	}

	var tc = NewCrawler()
	for _, e := range spiderMuxTests {
		tc.Handle(e.pattern, HandlerFunc(func(c chan<- Item, res *http.Response) {
			c <- res.StatusCode
		}))
	}
	for _, fake := range serveFakes {
		r := &http.Request{
			Method: "GET",
			Host:   fake.host,
			URL: &url.URL{
				Path: fake.path,
			},
		}
		res := &http.Response{
			Request:    r,
			StatusCode: fake.code,
		}
		h, _ := tc.Handler(res)
		c := make(chan Item, 1)
		h.ServeSpider(c, res)
		if code := (<-c).(int); code != fake.code {
			t.Errorf("%s expected %d; got %d", fake.host+fake.path, fake.code, code)
		}
	}
}

func TestSpiderIdleTimeout(t *testing.T) {
	timeout := 10 * time.Millisecond
	spider := &spider{
		key:         "test",
		c:           &Crawler{},
		idleTimeout: timeout,
	}
	done := make(chan struct{})
	var (
		start time.Time
		end   time.Time
	)
	go func() {
		defer close(done)
		start = time.Now()
		spider.crawlLoop()
		end = time.Now()
	}()
	<-done
	if d := end.Sub(start); d < timeout {
		t.Errorf("spider's timeout expected <= %s; but %s", timeout, t)
	}
}

func TestCrawlerNilLogger(t *testing.T) {
	loggers := []Logger{
		log.New(os.Stdout, "", log.LstdFlags),
		nilLogger{},
	}
	tc := &Crawler{}
	for _, logger := range loggers {
		tc.ErrorLog = logger
		tc.logf("test logging")
	}
}
