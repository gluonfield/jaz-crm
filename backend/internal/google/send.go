package google

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Outgoing is an email written as plain text, sent with an HTML alternative
// and its sender's signature below it, as Gmail sends mail. A reply names the
// Gmail thread it joins in the sending mailbox and the Message-IDs it answers.
type Outgoing struct {
	From       Address
	To, Cc     []string
	Subject    string
	Body       string
	Signature  Signature
	ThreadID   string
	InReplyTo  string
	References []string
}

// Signature is what Gmail adds below new mail from an address: its HTML, and
// its text as a plain-text email shows it.
type Signature struct {
	HTML, Text string
}

// Identity is how Gmail presents mail sent as an address of the mailbox: the
// name in its From header and the signature below new mail.
type Identity struct {
	Name      string
	Signature Signature
}

// Identity reads the From name and signature Gmail uses for an address of the
// mailbox; Gmail exposes no other signature, such as one chosen for replies.
func (c *Client) Identity(ctx context.Context, address string) (Identity, error) {
	var raw struct{ DisplayName, Signature string }
	if err := c.get(ctx, c.gmail("settings/sendAs/"+url.PathEscape(address)), nil, &raw); err != nil {
		return Identity{}, err
	}
	return Identity{Name: raw.DisplayName, Signature: Signature{HTML: raw.Signature, Text: htmlText(raw.Signature)}}, nil
}

// Send sends a message from the mailbox and returns its Gmail id. It is never
// retried: a failed send may still have gone out.
func (c *Client) Send(ctx context.Context, m Outgoing) (string, error) {
	var raw bytes.Buffer
	header := func(name, value string) {
		if value != "" {
			fmt.Fprintf(&raw, "%s: %s\r\n", name, strings.NewReplacer("\r", " ", "\n", " ").Replace(value))
		}
	}
	plain := m.Body
	rich := `<div dir="ltr">` + strings.ReplaceAll(html.EscapeString(m.Body), "\n", "<br>") + `</div>`
	if m.Signature.HTML != "" {
		plain += "\n\n" + m.Signature.Text
		rich += `<br><div dir="ltr" class="gmail_signature">` + m.Signature.HTML + `</div>`
	}
	var body bytes.Buffer
	parts := multipart.NewWriter(&body)
	for _, p := range [][2]string{{"text/plain", plain}, {"text/html", rich}} {
		w, err := parts.CreatePart(textproto.MIMEHeader{"Content-Type": {p[0] + "; charset=UTF-8"}, "Content-Transfer-Encoding": {"base64"}})
		if err != nil {
			return "", err
		}
		wrapped(w, base64.StdEncoding.EncodeToString([]byte(p[1])))
	}
	if err := parts.Close(); err != nil {
		return "", err
	}
	header("From", (&mail.Address{Name: m.From.Name, Address: m.From.Email}).String())
	header("To", strings.Join(m.To, ", "))
	header("Cc", strings.Join(m.Cc, ", "))
	header("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	header("In-Reply-To", bracketed(m.InReplyTo))
	header("References", bracketed(m.References...))
	header("MIME-Version", "1.0")
	header("Content-Type", mime.FormatMediaType("multipart/alternative", map[string]string{"boundary": parts.Boundary()}))
	raw.WriteString("\r\n")
	raw.Write(body.Bytes())
	var out struct{ ID string }
	err := c.post(ctx, c.gmail("messages/send"), map[string]string{"raw": base64.URLEncoding.EncodeToString(raw.Bytes()), "threadId": m.ThreadID}, &out)
	return out.ID, err
}

// wrapped writes base64 in the 76-character lines mail requires.
func wrapped(w io.Writer, encoded string) {
	for len(encoded) > 76 {
		io.WriteString(w, encoded[:76]+"\r\n")
		encoded = encoded[76:]
	}
	io.WriteString(w, encoded+"\r\n")
}

// ThreadOf finds the Gmail thread holding the message with a Message-ID in
// this mailbox; ErrNotFound when the mailbox does not have it.
func (c *Client) ThreadOf(ctx context.Context, messageID string) (string, error) {
	var raw struct {
		Messages []struct{ ThreadID string }
	}
	if err := c.get(ctx, c.gmail("messages"), url.Values{"q": {"rfc822msgid:" + messageID}, "includeSpamTrash": {"true"}}, &raw); err != nil {
		return "", err
	}
	if len(raw.Messages) == 0 {
		return "", ErrNotFound
	}
	return raw.Messages[0].ThreadID, nil
}

func bracketed(ids ...string) string {
	var out []string
	for _, id := range ids {
		if id != "" {
			out = append(out, "<"+id+">")
		}
	}
	return strings.Join(out, " ")
}
