package google

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"strings"
	"testing"
)

func TestSendReply(t *testing.T) {
	sends := 0
	c := fake(t, map[string]http.HandlerFunc{
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
			text, _ := io.ReadAll(base64.NewDecoder(base64.StdEncoding, m.Body))
			got := []string{body.ThreadID, m.Header.Get("From"), m.Header.Get("To"), m.Header.Get("Cc"), subject, m.Header.Get("In-Reply-To"), m.Header.Get("References"), string(text)}
			want := []string{"t9", "owner@cas.dev", "jane@acme.com, sales@acme.com", "bob@acme.com", "Re: Quote for 500 brackets — revised", "<m2@acme.com>", "<m1@acme.com> <m2@acme.com>", strings.Repeat("Thanks Jane. ", 10)}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("sent field %d = %q, want %q", i, got[i], want[i])
				}
			}
			if sends == 1 {
				io.WriteString(w, `{"id":"s1","threadId":"t9"}`)
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	})
	reply := Outgoing{
		From: "owner@cas.dev", To: []string{"jane@acme.com", "sales@acme.com"}, Cc: []string{"bob@acme.com"},
		Subject: "Re: Quote for 500 brackets — revised", Body: strings.Repeat("Thanks Jane. ", 10),
		ThreadID: "t9", InReplyTo: "m2@acme.com", References: []string{"m1@acme.com", "m2@acme.com"},
	}
	if id, err := c.Send(t.Context(), reply); err != nil || id != "s1" {
		t.Fatalf("send: %q %v", id, err)
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
