package google

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMessage(t *testing.T) {
	c := fake(t, map[string]http.HandlerFunc{
		"/gmail/v1/users/me/messages/gone": respond(http.StatusNotFound, `{"error":{"code":404,"message":"Not Found"}}`),
		"/gmail/v1/users/me/messages/m1": func(w http.ResponseWriter, r *http.Request) {
			wantQuery(t, r.URL, url.Values{"format": {"metadata"}, "metadataHeaders": {"From", "Reply-To", "To", "Cc", "Subject", "Message-ID", "In-Reply-To", "References", "List-Unsubscribe", "Precedence", "Auto-Submitted", "Delivered-To"}})
			io.WriteString(w, `{"id":"m1","threadId":"t1","labelIds":["INBOX","UNREAD"],"internalDate":"1760000000123","payload":{"headers":[
				{"name":"From","value":"\"Doe, Jane\" <Jane@Example.com>"},
				{"name":"Reply-To","value":"Sales <Sales@Example.com>"},
				{"name":"To","value":"bob@x.com, 'Carol' <Carol@X.com>"},
				{"name":"Cc","value":"Acme [Support] <Support@Acme.com>, Doe, John <john@x.com>"},
				{"name":"Subject","value":"Intro"},
				{"name":"Message-Id","value":" <abc.123@mail.x.com> "},
				{"name":"In-Reply-To","value":"<prev@x.com>"},
				{"name":"References","value":"<r1@x.com>\r\n <r2@x.com><prev@x.com>"},
				{"name":"Delivered-To","value":"Owner@X.com"},
				{"name":"Delivered-To","value":" jane.alias@x.com "}]}}`)
		},
		"/gmail/v1/users/me/messages/m2": func(w http.ResponseWriter, r *http.Request) {
			wantQuery(t, r.URL, url.Values{"format": {"full"}})
			fmt.Fprintf(w, `{"id":"m2","internalDate":"0","payload":{"mimeType":"multipart/mixed","parts":[
				{"mimeType":"multipart/alternative","parts":[
					{"mimeType":"text/plain","body":{"data":%q}},
					{"mimeType":"text/html","body":{"data":%q}}]},
				{"mimeType":"text/plain","body":{"data":%q}},
				{"mimeType":"text/plain","filename":"notes.txt","body":{"data":%q}}]}}`,
				base64.URLEncoding.EncodeToString([]byte("Hello Ann!")),
				base64.URLEncoding.EncodeToString([]byte("<p>Hello Ann!</p>")),
				base64.RawURLEncoding.EncodeToString([]byte("-- footer.")),
				base64.URLEncoding.EncodeToString([]byte("attachment")))
		},
	})
	got, err := c.Messages(t.Context(), []string{"gone", "m1"}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := Message{
		ID:          "m1",
		ThreadID:    "t1",
		Labels:      []string{"INBOX", "UNREAD"},
		Date:        time.UnixMilli(1760000000123).UTC(),
		From:        Address{Name: "Doe, Jane", Email: "jane@example.com"},
		ReplyTo:     []Address{{Name: "Sales", Email: "sales@example.com"}},
		To:          []Address{{Email: "bob@x.com"}, {Name: "Carol", Email: "carol@x.com"}},
		Cc:          []Address{{Name: "Acme [Support]", Email: "support@acme.com"}, {Name: "John", Email: "john@x.com"}},
		Subject:     "Intro",
		MessageID:   "abc.123@mail.x.com",
		InReplyTo:   "prev@x.com",
		References:  []string{"r1@x.com", "r2@x.com", "prev@x.com"},
		DeliveredTo: []string{"owner@x.com", "jane.alias@x.com"},
	}
	if !reflect.DeepEqual(got, []Message{{}, want}) {
		t.Errorf("metadata messages =\n%+v\nwant a deleted one empty, then\n%+v", got, want)
	}
	full, err := c.Messages(t.Context(), []string{"m2"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Hello Ann!\n\n-- footer."; full[0].Text != want {
		t.Errorf("text = %q, want %q", full[0].Text, want)
	}
	if full[0].HTML != "<p>Hello Ann!</p>" {
		t.Errorf("HTML alternative = %q", full[0].HTML)
	}
}

func TestHTMLOnlyText(t *testing.T) {
	src := `<html><head><title>Newsletter</title><style>p{color:red}</style></head><body>
<div>Hello&nbsp;<b>Ann</b>,</div><script>document.write("x")</script>
<p>1 &lt; 2</p><p></p><br>
<table><tr><td>A</td><td>B</td></tr></table></body></html>`
	var p part
	payload := fmt.Sprintf(`{"mimeType":"multipart/alternative","parts":[{"mimeType":"text/html","body":{"data":%q}}]}`, base64.URLEncoding.EncodeToString([]byte(src)))
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		t.Fatal(err)
	}
	got, rich := p.content()
	if want := "Hello Ann,\n\n1 < 2\n\nA\n\nB"; got != want || rich != src {
		t.Errorf("text = %q, want %q", got, want)
		t.Errorf("HTML = %q, want original body", rich)
	}
}

func TestTextIsValidAndCapped(t *testing.T) {
	for raw, want := range map[string]string{
		"caf\xe9":                      "caf�",
		strings.Repeat("é", maxText+1): strings.Repeat("é", maxText),
	} {
		p := part{MimeType: "text/plain", Body: struct{ Data string }{base64.RawURLEncoding.EncodeToString([]byte(raw))}}
		if got, rich := p.content(); got != want || rich != "" {
			t.Errorf("text(%.10q) = %.10q (%d runes), want %.10q", raw, got, len([]rune(got)), want)
		}
	}
}

// A body in the charset its Content-Type names reads as UTF-8.
func TestCharset(t *testing.T) {
	var p part
	payload := fmt.Sprintf(`{"mimeType":"text/plain","headers":[{"name":"Content-Type","value":"text/plain; charset=ISO-8859-1"}],"body":{"data":%q}}`,
		base64.RawURLEncoding.EncodeToString([]byte("Caf\xe9 cr\xe8me")))
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.content(); got != "Café crème" {
		t.Errorf("text = %q", got)
	}
}

func TestBulk(t *testing.T) {
	cases := []struct {
		headers map[string]string
		want    bool
	}{
		{map[string]string{"precedence": "first-class"}, false},
		{map[string]string{"list-unsubscribe": "<mailto:u@x.com>"}, true},
		{map[string]string{"precedence": "Bulk"}, true},
		{map[string]string{"auto-submitted": "no"}, false},
		{map[string]string{"auto-submitted": "auto-replied"}, true},
	}
	for _, tc := range cases {
		if got := bulk(tc.headers); got != tc.want {
			t.Errorf("bulk(%v) = %v, want %v", tc.headers, got, tc.want)
		}
	}
}

func TestHistory(t *testing.T) {
	c := fake(t, map[string]http.HandlerFunc{
		"/gmail/v1/users/me/history": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("startHistoryId") == "1" {
				respond(http.StatusNotFound, `{"error":{"code":404,"message":"Requested entity was not found."}}`)(w, r)
				return
			}
			wantQuery(t, r.URL, url.Values{"startHistoryId": {"7"}, "historyTypes": {"messageAdded"}, "pageToken": {"p2"}, "maxResults": {"500"}})
			io.WriteString(w, `{"history":[{"messagesAdded":[{"message":{"id":"b"}},{"message":{"id":"a"}}]},{"messagesAdded":[{"message":{"id":"b"}}]}],"historyId":"9"}`)
		},
	})
	page, err := c.History(t.Context(), "7", "p2")
	if err != nil {
		t.Fatal(err)
	}
	if want := (HistoryPage{MessageIDs: []string{"a", "b"}, HistoryID: "9"}); !reflect.DeepEqual(page, want) {
		t.Errorf("page = %+v, want %+v", page, want)
	}
	if _, err := c.History(t.Context(), "1", ""); !errors.Is(err, ErrExpiredCursor) {
		t.Errorf("err = %v, want ErrExpiredCursor", err)
	}
}

func TestWatch(t *testing.T) {
	c := fake(t, map[string]http.HandlerFunc{
		"/gmail/v1/users/me/watch": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if want := `{"labelFilterBehavior":"include","labelIds":["INBOX","SENT"],"topicName":"projects/p/topics/gmail"}`; string(body) != want {
				t.Errorf("body = %s, want %s", body, want)
			}
			io.WriteString(w, `{"historyId":"42","expiration":"1760000000000"}`)
		},
	})
	historyID, expiration, err := c.Watch(t.Context(), "projects/p/topics/gmail")
	if err != nil || historyID != "42" || !expiration.Equal(time.UnixMilli(1760000000000)) {
		t.Errorf("Watch = %q, %v, %v", historyID, expiration, err)
	}
}
