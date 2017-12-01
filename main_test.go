package antch

import (
	"net/http"
)

// backMessageHandler returns a HttpMessageHandler that use
// custom http.Client
func backMessageHandler(c *http.Client) HttpMessageHandler {
	return HttpMessageHandlerFunc(func(req *http.Request) (*http.Response, error) {
		return c.Do(req)
	})
}

// defaultMessageHandler returns a HttpMessageHandler that use
// http.DefaultClient.
func defaultMessageHandler() HttpMessageHandler {
	return backMessageHandler(http.DefaultClient)
}
