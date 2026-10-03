package google

import (
	"strings"

	"golang.org/x/net/html"
)

// HasDraftText checks authored text without changing the original MIME body.
func (m Message) HasDraftText() bool {
	if m.HTML != "" {
		root, err := html.Parse(strings.NewReader(m.HTML))
		if err == nil {
			return draftText(root)
		}
	}
	for line := range strings.Lines(m.Text) {
		line = strings.TrimSpace(line)
		if line == "--" {
			break
		}
		if line != "" && !strings.HasPrefix(line, ">") {
			return true
		}
	}
	return false
}

func draftText(node *html.Node) bool {
	if node.Type == html.ElementNode {
		switch node.Data {
		case "head", "script", "style", "blockquote":
			return false
		}
		for _, attribute := range node.Attr {
			if attribute.Key == "class" {
				for _, class := range strings.Fields(attribute.Val) {
					switch class {
					case "gmail_signature", "gmail_signature_prefix", "gmail_quote":
						return false
					}
				}
			}
		}
	}
	if node.Type == html.TextNode && strings.TrimSpace(node.Data) != "" {
		return true
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if draftText(child) {
			return true
		}
	}
	return false
}
