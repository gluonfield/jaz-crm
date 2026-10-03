package google

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

type part struct {
	MimeType, Filename string
	Headers            []struct{ Name, Value string }
	Body               struct{ Data string }
	Parts              []part
}

func (p part) content() (string, string) {
	bodies := map[string][]string{}
	p.collect(bodies)
	rich := strings.ToValidUTF8(strings.Join(bodies["text/html"], "\n"), "�")
	text := strings.TrimSpace(strings.Join(bodies["text/plain"], "\n\n"))
	if text == "" {
		text = htmlText(rich)
	}
	text = strings.ToValidUTF8(text, "�")
	return text, rich
}

func (p part) collect(bodies map[string][]string) {
	if p.Filename != "" {
		return
	}
	if p.Body.Data != "" {
		data, _ := base64.RawURLEncoding.DecodeString(strings.TrimRight(p.Body.Data, "="))
		mimeType := strings.ToLower(p.MimeType)
		bodies[mimeType] = append(bodies[mimeType], p.decode(data))
	}
	for _, child := range p.Parts {
		child.collect(bodies)
	}
}

// decode converts a body from the charset its Content-Type names to UTF-8.
func (p part) decode(data []byte) string {
	for _, h := range p.Headers {
		if !strings.EqualFold(h.Name, "Content-Type") {
			continue
		}
		_, params, _ := mime.ParseMediaType(h.Value)
		if label := params["charset"]; label != "" && !strings.EqualFold(label, "utf-8") {
			if r, err := charset.NewReaderLabel(label, bytes.NewReader(data)); err == nil {
				if converted, err := io.ReadAll(r); err == nil {
					return string(converted)
				}
			}
		}
	}
	return string(data)
}

// htmlText keeps an HTML body's text, its list items as "- " lines and the
// web address of each link after the link's text.
func htmlText(src string) string {
	z := html.NewTokenizer(strings.NewReader(src))
	var b strings.Builder
	skip := false
	href, start := "", 0
	for {
		token := z.Next()
		switch token {
		case html.ErrorToken:
			return tidy(b.String())
		case html.TextToken:
			if !skip {
				b.WriteString(strings.ReplaceAll(string(z.Text()), "\n", " "))
			}
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			name, attrs := z.TagName()
			switch string(name) {
			case "script", "style", "title":
				skip = token == html.StartTagToken
			case "li":
				if token == html.StartTagToken {
					b.WriteString("\n- ")
				}
			case "a":
				if token == html.StartTagToken {
					href, start = linkTarget(z, attrs), b.Len()
				} else if href != "" && !strings.Contains(b.String()[start:], href) {
					b.WriteString(" " + href)
				}
			case "br", "p", "div", "tr", "td", "th", "table", "ul", "ol", "hr", "blockquote", "pre",
				"h1", "h2", "h3", "h4", "h5", "h6", "section", "article", "header", "footer":
				b.WriteByte('\n')
			}
		}
	}
}

// linkTarget is the web address an anchor opens, if it opens one.
func linkTarget(z *html.Tokenizer, more bool) string {
	for more {
		var key, value []byte
		key, value, more = z.TagAttr()
		if string(key) == "href" && (strings.HasPrefix(string(value), "https://") || strings.HasPrefix(string(value), "http://")) {
			return string(value)
		}
	}
	return ""
}

// tidy collapses whitespace within lines and runs of blank lines into one.
func tidy(s string) string {
	var lines []string
	for line := range strings.Lines(s) {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" || len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (p part) attachments() []string {
	var names []string
	if p.Filename != "" {
		names = append(names, p.Filename)
	}
	for _, child := range p.Parts {
		names = append(names, child.attachments()...)
	}
	return names
}
