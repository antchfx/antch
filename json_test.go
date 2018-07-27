package antch

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

var testJSON = `{"name":"John","age":31,"city":"New York"}`

func TestParseJSON(t *testing.T) {
	res := &http.Response{
		Body: ioutil.NopCloser(strings.NewReader(testJSON)),
	}
	_, err := ParseJSON(res)
	if err != nil {
		t.Fatal(err)
	}
}
