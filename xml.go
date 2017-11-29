package antch

import (
	"net/http"

	"github.com/antchfx/xquery/xml"
)

// ParseXML parses an HTTP response as XML document.
func ParseXML(resp *http.Response) (*xmlquery.Node, error) {
	return xmlquery.Parse(resp.Body)
}
