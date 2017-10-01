package antch

import (
	"bytes"
	"io"
	"net/http"

	"github.com/antchfx/xquery/html"
	"github.com/antchfx/xquery/xml"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

// ParseHTML parses an HTTP response as HTML document.
func ParseHTML(resp *http.Response) (*html.Node, error) {
	var (
		ce encoding.Encoding
		r  io.Reader = resp.Body
	)

	mediatype := ParseMediaType(resp.Header.Get("Content-Type"))
	if mediatype.Charset == "" {
		// If response HTTP header not include charset.
		preview := make([]byte, 1024)
		n, err := io.ReadFull(r, preview)
		switch {
		case err == io.ErrUnexpectedEOF:
			preview = preview[:n]
			r = bytes.NewReader(preview)
		case err != nil:
			return nil, err
		default:
			r = io.MultiReader(bytes.NewReader(preview), r)
		}

		ce, _, _ = charset.DetermineEncoding(preview, "")
	} else {
		e, err := htmlindex.Get(mediatype.Charset)
		if err != nil {
			return nil, err
		}
		ce = e
	}

	if ce != encoding.Nop {
		r = transform.NewReader(r, ce.NewDecoder())
	}
	return htmlquery.Parse(r)
}

// ParseXML parses an HTTP response as XML document.
func ParseXML(resp *http.Response) (*xmlquery.Node, error) {
	return xmlquery.Parse(resp.Body)
}
