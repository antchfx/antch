package antch

import (
	"io"
	"io/ioutil"
	"net/http"
)

// Handler is the HTTP Response handler interface that defines
// how to extract scraped items from their pages.
//
// ServeSpider should be write got Item to the Channel.
type Handler interface {
	ServeSpider(chan<- Item, *http.Response)
}

// HandlerFunc is an adapter to allow the use of ordinary
// functions as Spider.
type HandlerFunc func(chan<- Item, *http.Response)

// ServeSpider performs extract data from received HTTP response and
// write it into the Channel c.
func (f HandlerFunc) ServeSpider(c chan<- Item, resp *http.Response) {
	f(c, resp)
}

// VoidHandler returns a Handler that without doing anything.
func VoidHandler() Handler {
	return HandlerFunc(func(_ chan<- Item, resp *http.Response) {
		// https://stackoverflow.com/questions/17948827/reusing-http-connections-in-golang
		io.Copy(ioutil.Discard, resp.Body)
	})
}
