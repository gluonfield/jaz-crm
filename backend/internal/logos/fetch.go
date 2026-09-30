package logos

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"golang.org/x/net/html"
)

// Fetcher reads icons from websites.
type Fetcher struct {
	Client *http.Client
	// Home maps a domain to its home page.
	Home func(domain string) string
}

// NewFetcher reaches public addresses only, so a domain named in mail cannot
// point the server at its own network.
func NewFetcher() Fetcher {
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: public}
	return Fetcher{
		Client: &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{DialContext: dialer.DialContext, TLSHandshakeTimeout: 5 * time.Second}},
		Home:   func(domain string) string { return "https://" + domain + "/" },
	}
}

func public(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return fmt.Errorf("logos: %s is not a public address", host)
	}
	return nil
}

const (
	pageLimit  = 512 << 10
	imageLimit = 256 << 10
)

// Icon takes the largest icon a site's home page declares, else its
// favicon.ico. A site without one, or out of reach, gives no image.
func (f Fetcher) Icon(ctx context.Context, domain string) storage.Logo {
	logo := storage.Logo{Domain: domain}
	home, err := url.Parse(f.Home(domain))
	if err != nil {
		return logo
	}
	var candidates []string
	if res, page, err := f.get(ctx, home.String()); err == nil {
		home = res.Request.URL
		candidates = icons(home, page)
	}
	candidates = append(candidates, home.ResolveReference(&url.URL{Path: "/favicon.ico"}).String())
	for _, c := range candidates {
		res, body, err := f.get(ctx, c)
		if err != nil || len(body) > imageLimit {
			continue
		}
		if kind := imageType(res.Header.Get("Content-Type"), body); kind != "" {
			logo.ContentType, logo.Image = kind, body
			return logo
		}
	}
	return logo
}

// get reads up to pageLimit bytes, one more than it keeps.
func (f Fetcher) get(ctx context.Context, target string) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "JazCRM (+https://github.com/gluonfield/jaz-crm)")
	res, err := f.Client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("logos: %s: %s", target, res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, pageLimit+1))
	return res, body, err
}

// imageType trusts the bytes, not the server, except for SVG, which only a
// declared type and an svg element identify.
func imageType(declared string, body []byte) string {
	if sniffed := http.DetectContentType(body); strings.HasPrefix(sniffed, "image/") {
		return sniffed
	}
	if strings.HasPrefix(declared, "image/svg+xml") && bytes.Contains(body, []byte("<svg")) {
		return "image/svg+xml"
	}
	return ""
}

// icons lists the icons a page's head declares, largest first: an SVG
// scales, and an apple-touch-icon without sizes is 180 pixels.
func icons(base *url.URL, page []byte) []string {
	type icon struct {
		href string
		size int
	}
	var found []icon
	z := html.NewTokenizer(bytes.NewReader(page))
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			break
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		name, more := z.TagName()
		if string(name) == "body" {
			break
		}
		if string(name) != "link" {
			continue
		}
		attrs := map[string]string{}
		for more {
			var key, value []byte
			key, value, more = z.TagAttr()
			attrs[string(key)] = string(value)
		}
		rels := strings.Fields(strings.ToLower(attrs["rel"]))
		touch := slices.Contains(rels, "apple-touch-icon") || slices.Contains(rels, "apple-touch-icon-precomposed")
		href, err := base.Parse(attrs["href"])
		if attrs["href"] == "" || err != nil || !touch && !slices.Contains(rels, "icon") {
			continue
		}
		size := 16
		if touch {
			size = 180
		}
		for _, s := range strings.Fields(strings.ToLower(attrs["sizes"])) {
			if w, _, ok := strings.Cut(s, "x"); ok {
				if n, err := strconv.Atoi(w); err == nil {
					size = max(size, n)
				}
			}
		}
		if attrs["type"] == "image/svg+xml" || strings.HasSuffix(href.Path, ".svg") {
			size = 1024
		}
		found = append(found, icon{href.String(), size})
	}
	slices.SortStableFunc(found, func(a, b icon) int { return b.size - a.size })
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.href
	}
	return out
}
