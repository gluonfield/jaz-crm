package google

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// EditDraft preserves MIME attachments and untouched HTML. Imported signatures
// already belong to the body, so editing never appends an account signature.
func EditDraft(raw []byte, original Message, next Outgoing) ([]byte, error) {
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(message.Body)
	if err != nil {
		return nil, err
	}
	headers := textproto.MIMEHeader(message.Header)
	if next.Body != original.Text {
		seen := map[string]bool{}
		body, err = editDraftBody(headers, body, next.Body, seen)
		if err != nil {
			return nil, err
		}
		if len(seen) == 0 {
			return nil, fmt.Errorf("this draft has no editable text body; edit it in Gmail")
		}
	}
	for name, value := range map[string]string{"Subject": mime.QEncoding.Encode("utf-8", next.Subject), "To": strings.Join(next.To, ", "), "Cc": strings.Join(next.Cc, ", ")} {
		if strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("invalid %s header", name)
		}
		headers.Set(name, value)
	}
	return joinMIME(headers, body), nil
}

func editDraftBody(headers textproto.MIMEHeader, body []byte, text string, seen map[string]bool) ([]byte, error) {
	contentType := headers.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}
	kind, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("read draft content type: %w", err)
	}
	disposition, dispositionParams, _ := mime.ParseMediaType(headers.Get("Content-Disposition"))
	if disposition == "attachment" || dispositionParams["filename"] != "" || params["name"] != "" {
		return body, nil
	}
	switch kind {
	case "multipart/signed", "multipart/encrypted":
		return nil, fmt.Errorf("edit signed or encrypted drafts in Gmail before reviewing them here")
	case "text/plain", "text/html":
		if seen[kind] {
			return nil, fmt.Errorf("this draft has multiple %s body parts; edit it in Gmail", kind)
		}
		seen[kind] = true
		if kind == "text/html" {
			text = `<div dir="ltr">` + strings.ReplaceAll(html.EscapeString(text), "\n", "<br>") + `</div>`
		}
		headers.Set("Content-Type", kind+"; charset=UTF-8")
		headers.Set("Content-Transfer-Encoding", "base64")
		var out bytes.Buffer
		wrapped(&out, base64.StdEncoding.EncodeToString([]byte(text)))
		return out.Bytes(), nil
	}
	if !strings.HasPrefix(kind, "multipart/") {
		return body, nil
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	var out bytes.Buffer
	writer := multipart.NewWriter(&out)
	if err := writer.SetBoundary(params["boundary"]); err != nil {
		return nil, err
	}
	for {
		part, err := reader.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		content, err = editDraftBody(part.Header, content, text, seen)
		if err != nil {
			return nil, err
		}
		destination, err := writer.CreatePart(part.Header)
		if err != nil {
			return nil, err
		}
		if _, err := destination.Write(content); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func joinMIME(headers textproto.MIMEHeader, body []byte) []byte {
	var out bytes.Buffer
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		for _, value := range headers[key] {
			fmt.Fprintf(&out, "%s: %s\r\n", key, value)
		}
	}
	out.WriteString("\r\n")
	out.Write(body)
	return out.Bytes()
}
