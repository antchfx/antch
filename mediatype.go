package antch

import (
	"fmt"
	"mime"
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
