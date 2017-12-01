package antch

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/antchfx/xquery/html"
)

func TestMediaTypeParse(t *testing.T) {
	s := "text/html; charset=utf-8"
	m := ParseMediaType(s)
	if v := m.ContentType(); v != s {
		t.Errorf("ContentType() = %s; want %s", v, s)
	}

	s = "text/html"
	m = ParseMediaType(s)
	if m.Charset != "" {
		t.Errorf("Charset = %s; want empty", m.Charset)
	}
	if g, e := m.Type, "text/html"; g != e {
		t.Errorf("Type = %s; want e", g, e)
	}
}

var testHTML = `<html><head><meta charset="utf-8"></head><body>abc,这是中文内容</body> </html>`

func TestParseHTML(t *testing.T) {
	res := &http.Response{
		Header: map[string][]string{
			"Content-Type": []string{"text/html; charset=utf-8"},
		},
		Body: ioutil.NopCloser(strings.NewReader(testHTML)),
	}
	doc, err := ParseHTML(res)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	body := htmlquery.FindOne(doc, "//body")
	if g, e := strings.TrimSpace(htmlquery.InnerText(body)), "abc,这是中文内容"; g != e {
		t.Errorf("body expected is %s; but got %s", e, g)
	}
}

func TestParseHTMLWithoutEncoding(t *testing.T) {
	res := &http.Response{
		Header: map[string][]string{
			"Content-Type": []string{"text/html"},
		},
		Body: ioutil.NopCloser(strings.NewReader(testHTML)),
	}
	doc, err := ParseHTML(res)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}
	body := htmlquery.FindOne(doc, "//body")
	if g, e := strings.TrimSpace(htmlquery.InnerText(body)), "abc,这是中文内容"; g != e {
		t.Errorf("body expected is %s; but got %s", e, g)
	}
}
