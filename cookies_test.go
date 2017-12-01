package antch

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookiesHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("Flavor"); err != nil {
			http.SetCookie(w, &http.Cookie{Name: "Flavor", Value: "Chocolate Chip"})
		} else {
			cookie.Value = "Oatmeal Raisin"
			http.SetCookie(w, cookie)
		}
	}))
	defer ts.Close()

	handler := CookiesMiddleware()(defaultMessageHandler())
	req, _ := http.NewRequest("GET", ts.URL, nil)

	resp, err := handler.Send(req)
	if err != nil {
		t.Fatal(err)
	}

	// After 1st request.
	if len(resp.Cookies()) == 0 {
		t.Fatal("no cookies data after 1st request")
	}
	if g, e := resp.Cookies()[0].String(), "Flavor=Chocolate Chip"; e != g {
		t.Errorf("expected %s; got %s after 1st request", e, g)
	}

	resp, err = handler.Send(req)
	if err != nil {
		t.Fatal(err)
	}
	// After 2nd request.
	if len(resp.Cookies()) == 0 {
		t.Fatal("no cookies data after 2nd request")
	}
	if g, e := resp.Cookies()[0].String(), "Flavor=Oatmeal Raisin"; e != g {
		t.Errorf("expected %s; got %s after 2nd request", e, g)
	}
}
