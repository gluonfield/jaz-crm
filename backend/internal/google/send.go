package google

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/url"
	"strings"
)

// Outgoing is a plain-text email. A reply names the Gmail thread it joins in
// the sending mailbox and the Message-IDs it answers.
type Outgoing struct {
	From       string
	To, Cc     []string
	Subject    string
	Body       string
	ThreadID   string
	InReplyTo  string
	References []string
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
	header("From", m.From)
	header("To", strings.Join(m.To, ", "))
	header("Cc", strings.Join(m.Cc, ", "))
	header("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	header("In-Reply-To", bracketed(m.InReplyTo))
	header("References", bracketed(m.References...))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=UTF-8")
	header("Content-Transfer-Encoding", "base64")
	raw.WriteString("\r\n")
	body := base64.StdEncoding.EncodeToString([]byte(m.Body))
	for len(body) > 76 {
		raw.WriteString(body[:76] + "\r\n")
		body = body[76:]
	}
	raw.WriteString(body + "\r\n")
	var out struct{ ID string }
	err := c.post(ctx, c.gmail("messages/send"), map[string]string{"raw": base64.URLEncoding.EncodeToString(raw.Bytes()), "threadId": m.ThreadID}, &out)
	return out.ID, err
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
