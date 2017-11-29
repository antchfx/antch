package antch

import (
	"net/http"
)

// HttpMessageHandler is an interface that receives an HTTP request
// and returns an HTTP response.
type HttpMessageHandler interface {
	Send(*http.Request) (*http.Response, error)
}

// Middleware is the HTTP message transport middle layer that send
// HTTP request passed one message Handler to the next message Handler
// until returns an HTTP response.
type Middleware func(HttpMessageHandler) HttpMessageHandler

// HttpMessageHandlerFunc is an adapter to allow the use of ordinary
// functions as HttpMessageHandler.
type HttpMessageHandlerFunc func(*http.Request) (*http.Response, error)

// Send sends a HTTP request and receives HTTP response.
func (f HttpMessageHandlerFunc) Send(req *http.Request) (*http.Response, error) {
	return f(req)
}
