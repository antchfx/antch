package antch

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

var testXML = `<?xml version="1.0" encoding="UTF-8"?><root></root>`

func TestParseXML(t *testing.T) {
	res := &http.Response{
		Body: ioutil.NopCloser(strings.NewReader(testXML)),
	}
	_, err := ParseXML(res)
	if err != nil {
		t.Fatal(err)
	}
}
