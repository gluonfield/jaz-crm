package google

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Draft struct {
	ID      string
	Message Message
	Raw     []byte
}

type DraftPage struct {
	Drafts []Draft
	Next   string
}

func (c *Client) Drafts(ctx context.Context, pageToken string) (DraftPage, error) {
	var raw struct {
		Drafts []struct {
			ID      string
			Message gmailMessage
		}
		NextPageToken string
	}
	if err := c.get(ctx, c.gmail("drafts"), url.Values{"pageToken": {pageToken}, "maxResults": {"500"}}, &raw); err != nil {
		return DraftPage{}, err
	}
	out := DraftPage{Next: raw.NextPageToken}
	for _, d := range raw.Drafts {
		out.Drafts = append(out.Drafts, Draft{ID: d.ID, Message: d.Message.message()})
	}
	return out, nil
}

func (c *Client) Draft(ctx context.Context, id string) (Draft, error) {
	var raw struct {
		ID      string
		Message gmailMessage
	}
	if err := c.get(ctx, c.gmail("drafts/"+url.PathEscape(id)), url.Values{"format": {"full"}}, &raw); err != nil {
		return Draft{}, err
	}
	return Draft{ID: raw.ID, Message: raw.Message.message()}, nil
}

func (c *Client) DraftRaw(ctx context.Context, id string) (Draft, error) {
	var raw struct {
		ID      string
		Message gmailMessage
	}
	if err := c.get(ctx, c.gmail("drafts/"+url.PathEscape(id)), url.Values{"format": {"raw"}}, &raw); err != nil {
		return Draft{}, err
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw.Message.Raw)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(raw.Message.Raw)
	}
	return Draft{ID: raw.ID, Message: raw.Message.message(), Raw: decoded}, err
}

func (c *Client) ThreadMessages(ctx context.Context, id string) ([]Message, error) {
	var raw struct{ Messages []gmailMessage }
	if err := c.get(ctx, c.gmail("threads/"+url.PathEscape(id)), url.Values{"format": {"full"}}, &raw); err != nil {
		return nil, err
	}
	result := make([]Message, len(raw.Messages))
	for i, message := range raw.Messages {
		result[i] = message.message()
	}
	return result, nil
}

func (c *Client) UpdateDraft(ctx context.Context, id, threadID string, raw []byte) (Draft, error) {
	body, err := json.Marshal(draftPayload(id, threadID, raw))
	if err != nil {
		return Draft{}, err
	}
	var result struct {
		ID      string
		Message gmailMessage
	}
	err = c.do(ctx, http.MethodPut, c.gmail("drafts/"+url.PathEscape(id)), bytes.NewReader(body), &result)
	return Draft{ID: result.ID, Message: result.Message.message()}, err
}

// SendDraft consumes the existing provider identity. It never creates a
// replacement draft or retries an uncertain send.
func (c *Client) SendDraft(ctx context.Context, id, threadID string, raw []byte) (string, error) {
	var sent struct{ ID string }
	err := c.post(ctx, c.gmail("drafts/send"), draftPayload(id, threadID, raw), &sent)
	if err == nil && sent.ID == "" {
		err = fmt.Errorf("Gmail returned no sent message ID")
	}
	return sent.ID, err
}

func draftPayload(id, threadID string, raw []byte) any {
	type message struct {
		Raw      string `json:"raw"`
		ThreadID string `json:"threadId,omitempty"`
	}
	out := struct {
		ID      string   `json:"id"`
		Message *message `json:"message,omitempty"`
	}{ID: id}
	if raw != nil {
		out.Message = &message{Raw: base64.RawURLEncoding.EncodeToString(raw), ThreadID: threadID}
	}
	return out
}
