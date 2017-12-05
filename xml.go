package antch

import (
	"net/http"

	"github.com/antchfx/xmlquery"
)

// ParseXML parses an HTTP response as XML document.
func ParseXML(resp *http.Response) (*xmlquery.Node, error) {
	return xmlquery.Parse(resp.Body)
}
