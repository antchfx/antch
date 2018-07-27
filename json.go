package antch

import (
	"net/http"

	"github.com/antchfx/jsonquery"
)

// ParseJSON parses an HTTP response as JSON document.
func ParseJSON(resp *http.Response) (*jsonquery.Node, error) {
	r, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	return jsonquery.Parse(r)
}
