package google

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

func TestEditDraftRejectsAmbiguousBodies(t *testing.T) {
	for _, kind := range []string{"text/plain", "text/html"} {
		t.Run(kind, func(t *testing.T) {
			raw := []byte(fmt.Sprintf("Subject: Original\r\nContent-Type: multipart/mixed; boundary=outer\r\n\r\n--outer\r\nContent-Type: %s\r\n\r\nFirst segment\r\n--outer\r\nContent-Type: multipart/alternative; boundary=inner\r\n\r\n--inner\r\nContent-Type: %s\r\n\r\nSecond segment\r\n--inner--\r\n--outer--\r\n", kind, kind))
			original := Message{Text: "First segment\n\nSecond segment"}
			edited, err := EditDraft(raw, original, Outgoing{Subject: "Updated", Body: "New body"})
			if err == nil || edited != nil {
				t.Fatalf("ambiguous body must not produce sendable MIME: %q, %v", edited, err)
			}
			edited, err = EditDraft(raw, original, Outgoing{Subject: "Updated", Body: original.Text})
			if err != nil {
				t.Fatal(err)
			}
			message, err := mail.ReadMessage(bytes.NewReader(edited))
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(message.Body)
			if err != nil || !bytes.Equal(body, bytes.SplitN(raw, []byte("\r\n\r\n"), 2)[1]) {
				t.Fatalf("subject-only edit changed the MIME body: %q, %v", body, err)
			}
		})
	}
	raw := []byte("Content-Type: application/pdf\r\nContent-Disposition: attachment; filename=drawing.pdf\r\n\r\nPDF data")
	if edited, err := EditDraft(raw, Message{}, Outgoing{Body: "Add a message"}); err == nil || edited != nil {
		t.Fatalf("a body edit must not silently disappear: %q, %v", edited, err)
	}
}

func TestEditDraftKeepsAlternativeAndTextAttachment(t *testing.T) {
	raw := []byte(strings.ReplaceAll(`Subject: Original
Content-Type: multipart/mixed; boundary=outer

--outer
Content-Type: multipart/alternative; boundary=inner

--inner
Content-Type: text/plain; charset=UTF-8

Original body
--inner
Content-Type: text/html; charset=UTF-8

<b>Original body</b>
--inner--
--outer
Content-Type: text/plain; name=notes.txt
Content-Disposition: attachment; filename=notes.txt

Keep attachment verbatim
--outer--
`, "\n", "\r\n"))
	text := "New body <review>\nSecond line"
	edited, err := EditDraft(raw, Message{Text: "Original body"}, Outgoing{Subject: "Updated", Body: text})
	if err != nil {
		t.Fatal(err)
	}
	message, err := mail.ReadMessage(bytes.NewReader(edited))
	if err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(message.Body, params["boundary"])
	part, err := reader.NextRawPart()
	if err != nil {
		t.Fatal(err)
	}
	bodies := alternatives(t, &mail.Message{Header: mail.Header(part.Header), Body: part})
	if bodies["text/plain"] != text || bodies["text/html"] != `<div dir="ltr">New body &lt;review&gt;<br>Second line</div>` {
		t.Fatalf("edited alternatives: %#v", bodies)
	}
	part, err = reader.NextRawPart()
	if err != nil {
		t.Fatal(err)
	}
	attachment, err := io.ReadAll(part)
	if err != nil || string(attachment) != "Keep attachment verbatim" || part.FileName() != "notes.txt" {
		t.Fatalf("changed text attachment: %q, %v", attachment, err)
	}
}
