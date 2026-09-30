package logos_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/logos"
)

var (
	png = "\x89PNG\r\n\x1a\nrest"
	ico = "\x00\x00\x01\x00rest"
)

// site serves each domain's pages under /<domain>/.
func site(t *testing.T, pages map[string]string) logos.Fetcher {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if len(page) > 9 && page[:9] == "redirect:" {
			http.Redirect(w, r, page[9:], http.StatusFound)
			return
		}
		w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)
	return logos.Fetcher{Client: srv.Client(), Home: func(domain string) string { return srv.URL + "/" + domain + "/" }}
}

// The largest declared icon wins, relative to where the home page ended up;
// without one the favicon serves, and an unreachable site has none.
func TestIcon(t *testing.T) {
	f := site(t, map[string]string{
		"/acme.io/":             "redirect:/acme.io/en/",
		"/acme.io/en/":          `<html><head><link rel="icon" sizes="32x32" href="small.png"><link rel="apple-touch-icon" href="touch.png"></head><body><link rel="icon" href="late.png">`,
		"/acme.io/en/touch.png": png,
		"/acme.io/en/small.png": "\x89PNG\r\n\x1a\nsmall",
		"/favicon.ico":          ico,
		"/plain.io/":            `<html><head><title>Plain</title></head></html>`,
	})
	if logo := f.Icon(t.Context(), "acme.io"); logo.ContentType != "image/png" || string(logo.Image) != png {
		t.Errorf("acme: %q %q", logo.ContentType, logo.Image)
	}
	if logo := f.Icon(t.Context(), "plain.io"); logo.ContentType != "image/x-icon" || string(logo.Image) != ico {
		t.Errorf("plain: %q %q", logo.ContentType, logo.Image)
	}
	f.Home = func(domain string) string { return "http://127.0.0.1:1/" }
	if logo := f.Icon(t.Context(), "gone.io"); logo.Image != nil || logo.Domain != "gone.io" {
		t.Errorf("unreachable: %+v", logo)
	}
}

// A page declaring an HTML page as its icon, with no favicon, has no logo.
func TestIconIgnoresPagesPosingAsImages(t *testing.T) {
	f := site(t, map[string]string{
		"/fake.io/":     `<link rel="icon" href="/fake.io/icon">`,
		"/fake.io/icon": `<html>not an image</html>`,
	})
	if logo := f.Icon(t.Context(), "fake.io"); logo.Image != nil {
		t.Errorf("fake: %q", logo.ContentType)
	}
}

// The production fetcher refuses addresses on the server's own network.
func TestNewFetcherStaysPublic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(ico))
	}))
	t.Cleanup(srv.Close)
	f := logos.NewFetcher()
	f.Home = func(string) string { return srv.URL + "/" }
	if logo := f.Icon(t.Context(), "local.test"); logo.Image != nil {
		t.Fatal("fetched from a loopback address")
	}
}
