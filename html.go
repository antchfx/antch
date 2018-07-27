package antch

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

// MediaType describe the content type of an HTTP request or HTTP response.
type MediaType struct {
	// Type is the HTTP content type represents. such as
	// "text/html", "image/jpeg".
	Type string
	// Charset is the HTTP content encoding represents.
	Charset string
}

// ContentType returns the HTTP header content-type value.
func (m MediaType) ContentType() string {
	if len(m.Type) > 0 && m.Charset != "" {
		return fmt.Sprintf("%s; charset=%s", m.Type, m.Charset)
	}
	return m.Type
}

// ParseMediaType parsing a specified string v to MediaType struct.
func ParseMediaType(v string) MediaType {
	if v == "" {
		return MediaType{}
	}

	mimetype, params, err := mime.ParseMediaType(v)
	if err != nil {
		return MediaType{}
	}
	return MediaType{
		Type:    mimetype,
		Charset: params["charset"],
	}
}

func readResponseBody(resp *http.Response) (io.Reader, error) {
	var (
		ce encoding.Encoding
		r  io.Reader = resp.Body
	)

	mediatype := ParseMediaType(resp.Header.Get("Content-Type"))
	if mediatype.Charset == "" {
		// If HTTP Response's header not include a charset field,
		// reads 1024 bytes from Response body and geting encoding.
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
	return r, nil
}

// ParseHTML parses an HTTP response as HTML document.
func ParseHTML(resp *http.Response) (*html.Node, error) {
	r, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	return htmlquery.Parse(r)
}
