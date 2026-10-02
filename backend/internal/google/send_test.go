package google

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"strings"
	"testing"
)

// alternatives reads a sent message's plain-text and HTML bodies.
func alternatives(t *testing.T, m *mail.Message) map[string]string {
	t.Helper()
	kind, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || kind != "multipart/alternative" {
		t.Fatalf("content type %q: %v", m.Header.Get("Content-Type"), err)
	}
	out := map[string]string{}
	parts := multipart.NewReader(m.Body, params["boundary"])
	for {
		p, err := parts.NextRawPart()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		kind, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		text, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, p))
		if err != nil {
			t.Fatal(err)
		}
		out[kind] = string(text)
	}
}

func TestSendReply(t *testing.T) {
	sends := 0
	var bodies map[string]string
	c := fake(t, map[string]http.HandlerFunc{
		"/gmail/v1/users/me/settings/sendAs/owner@cas.dev": respond(http.StatusOK, `{"sendAsEmail":"owner@cas.dev","signature":"<div dir=\"ltr\"><b>Owner Name</b><div>CAS &amp; Co · <a href=\"https://cas.dev\">cas.dev</a></div></div>"}`),
		"/gmail/v1/users/me/messages/send": func(w http.ResponseWriter, r *http.Request) {
			sends++
			var body struct{ Raw, ThreadID string }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			raw, err := base64.URLEncoding.DecodeString(body.Raw)
			if err != nil {
				t.Fatal(err)
			}
			m, err := mail.ReadMessage(strings.NewReader(string(raw)))
			if err != nil {
				t.Fatal(err)
			}
			subject, _ := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
			got := []string{body.ThreadID, m.Header.Get("From"), m.Header.Get("To"), m.Header.Get("Cc"), subject, m.Header.Get("In-Reply-To"), m.Header.Get("References")}
			want := []string{"t9", "owner@cas.dev", "jane@acme.com, sales@acme.com", "bob@acme.com", "Re: Quote for 500 brackets — revised", "<m2@acme.com>", "<m1@acme.com> <m2@acme.com>"}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("sent field %d = %q, want %q", i, got[i], want[i])
				}
			}
			bodies = alternatives(t, m)
			if sends == 1 {
				io.WriteString(w, `{"id":"s1","threadId":"t9"}`)
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	})
	signature, err := c.Signature(t.Context(), "owner@cas.dev")
	if err != nil || signature.Text != "Owner Name\nCAS & Co · cas.dev https://cas.dev" {
		t.Fatalf("signature: %q %v", signature.Text, err)
	}
	draft := strings.Repeat("Thanks Jane. ", 10) + "\n500 < 600 & <b>soon</b>"
	reply := Outgoing{
		From: "owner@cas.dev", To: []string{"jane@acme.com", "sales@acme.com"}, Cc: []string{"bob@acme.com"},
		Subject: "Re: Quote for 500 brackets — revised", Body: draft, Signature: signature,
		ThreadID: "t9", InReplyTo: "m2@acme.com", References: []string{"m1@acme.com", "m2@acme.com"},
	}
	if id, err := c.Send(t.Context(), reply); err != nil || id != "s1" {
		t.Fatalf("send: %q %v", id, err)
	}
	if want := draft + "\n\n" + signature.Text; bodies["text/plain"] != want {
		t.Errorf("plain text = %q, want %q", bodies["text/plain"], want)
	}
	if html := bodies["text/html"]; !strings.Contains(html, "<br>500 &lt; 600 &amp; &lt;b&gt;soon&lt;/b&gt;</div>") || !strings.HasSuffix(html, `class="gmail_signature">`+signature.HTML+"</div>") {
		t.Errorf("HTML must escape the draft and end with the signature: %q", html)
	}
	if _, err := c.Send(t.Context(), reply); err == nil || sends != 2 {
		t.Fatalf("a failed send must be reported once, not retried: %d sends, %v", sends, err)
	}
}

func TestThreadOf(t *testing.T) {
	c := fake(t, map[string]http.HandlerFunc{
		"/gmail/v1/users/me/messages": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("q") == "rfc822msgid:m2@acme.com" {
				io.WriteString(w, `{"messages":[{"id":"x","threadId":"t9"}]}`)
				return
			}
			io.WriteString(w, `{"resultSizeEstimate":0}`)
		},
	})
	if thread, err := c.ThreadOf(t.Context(), "m2@acme.com"); err != nil || thread != "t9" {
		t.Fatalf("thread: %q %v", thread, err)
	}
	if _, err := c.ThreadOf(t.Context(), "other@acme.com"); err != ErrNotFound {
		t.Fatalf("a mailbox without the message: %v", err)
	}
}
