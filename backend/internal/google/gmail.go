package google

import (
	"context"
	"errors"
	"net/mail"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

type Address struct {
	Name, Email string
}

type Profile struct {
	Email, HistoryID string
}

type MessagePage struct {
	IDs  []string
	Next string
}

type Message struct {
	ID, ThreadID string
	Labels       []string
	Date         time.Time
	From         Address
	// ReplyTo is where the sender asks replies to go instead of From.
	ReplyTo    []Address
	To, Cc     []Address
	Subject    string
	MessageID  string
	InReplyTo  string
	References []string
	Bulk       bool
	Text, HTML string
	// DeliveredTo lists the mailboxes that received the message on its way
	// here, including those that forwarded it.
	DeliveredTo []string
}

type HistoryPage struct {
	MessageIDs      []string
	Next, HistoryID string
}

var metadataHeaders = []string{"From", "Reply-To", "To", "Cc", "Subject", "Message-ID", "In-Reply-To", "References", "List-Unsubscribe", "Precedence", "Auto-Submitted", "Delivered-To"}

func (c *Client) Profile(ctx context.Context) (Profile, error) {
	var raw struct {
		EmailAddress, HistoryID string
	}
	if err := c.get(ctx, c.gmail("profile"), nil, &raw); err != nil {
		return Profile{}, err
	}
	return Profile{Email: strings.ToLower(raw.EmailAddress), HistoryID: raw.HistoryID}, nil
}

func (c *Client) ListMessages(ctx context.Context, query, pageToken string) (MessagePage, error) {
	var raw struct {
		Messages      []struct{ ID string }
		NextPageToken string
	}
	q := url.Values{"q": {query}, "pageToken": {pageToken}, "maxResults": {"500"}, "includeSpamTrash": {"false"}}
	if err := c.get(ctx, c.gmail("messages"), q, &raw); err != nil {
		return MessagePage{}, err
	}
	page := MessagePage{Next: raw.NextPageToken}
	for _, m := range raw.Messages {
		page.IDs = append(page.IDs, m.ID)
	}
	return page, nil
}

// Messages reads up to messageReaders messages at once, starting messageRate a
// second: a read costs 5 of the 15,000 quota units Gmail allows a user a
// minute, and other clients of the account share them.
const (
	messageReaders = 8
	messageRate    = 10
)

// Messages returns messages in the order asked; one deleted since it was
// listed comes back empty. Text is empty unless full is set: metadata
// responses carry no bodies.
func (c *Client) Messages(ctx context.Context, ids []string, full bool) ([]Message, error) {
	messages := make([]Message, len(ids))
	errs := make([]error, len(ids))
	slots := make(chan struct{}, messageReaders)
	pace := time.NewTicker(time.Second / messageRate)
	defer pace.Stop()
	var wg sync.WaitGroup
	for i, id := range ids {
		<-pace.C
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			messages[i], errs[i] = c.message(ctx, id, full)
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return messages, nil
}

func (c *Client) message(ctx context.Context, id string, full bool) (Message, error) {
	q := url.Values{"format": {"metadata"}, "metadataHeaders": metadataHeaders}
	if full {
		q = url.Values{"format": {"full"}}
	}
	var raw struct {
		ID, ThreadID string
		LabelIDs     []string
		InternalDate int64 `json:",string"`
		Payload      part
	}
	if err := c.get(ctx, c.gmail("messages/"+url.PathEscape(id)), q, &raw); err != nil {
		return Message{}, err
	}
	h := map[string]string{}
	var delivered []string
	for _, header := range raw.Payload.Headers {
		name := strings.ToLower(header.Name)
		h[name] = strings.TrimSpace(header.Value)
		if name == "delivered-to" {
			delivered = append(delivered, strings.ToLower(h[name]))
		}
	}
	text, rich := raw.Payload.content()
	m := Message{
		ID:          raw.ID,
		ThreadID:    raw.ThreadID,
		Labels:      raw.LabelIDs,
		Date:        time.UnixMilli(raw.InternalDate).UTC(),
		ReplyTo:     addresses(h["reply-to"]),
		To:          addresses(h["to"]),
		Cc:          addresses(h["cc"]),
		Subject:     h["subject"],
		MessageID:   firstID(h["message-id"]),
		InReplyTo:   firstID(h["in-reply-to"]),
		References:  messageIDs(h["references"]),
		Bulk:        bulk(h),
		Text:        text,
		HTML:        rich,
		DeliveredTo: delivered,
	}
	if from := addresses(h["from"]); len(from) > 0 {
		m.From = from[0]
	}
	return m, nil
}

func (c *Client) History(ctx context.Context, startHistoryID, pageToken string) (HistoryPage, error) {
	var raw struct {
		History []struct {
			MessagesAdded []struct{ Message struct{ ID string } }
		}
		NextPageToken, HistoryID string
	}
	q := url.Values{"startHistoryId": {startHistoryID}, "historyTypes": {"messageAdded"}, "pageToken": {pageToken}, "maxResults": {"500"}}
	err := c.get(ctx, c.gmail("history"), q, &raw)
	if errors.Is(err, ErrNotFound) {
		return HistoryPage{}, ErrExpiredCursor
	}
	if err != nil {
		return HistoryPage{}, err
	}
	var ids []string
	for _, h := range raw.History {
		for _, added := range h.MessagesAdded {
			ids = append(ids, added.Message.ID)
		}
	}
	slices.Sort(ids)
	return HistoryPage{MessageIDs: slices.Compact(ids), Next: raw.NextPageToken, HistoryID: raw.HistoryID}, nil
}

func (c *Client) Watch(ctx context.Context, topic string) (historyID string, expiration time.Time, err error) {
	body := map[string]any{"topicName": topic, "labelIds": []string{"INBOX", "SENT"}, "labelFilterBehavior": "include"}
	var raw struct {
		HistoryID  string
		Expiration int64 `json:",string"`
	}
	if err := c.post(ctx, c.gmail("watch"), body, &raw); err != nil {
		return "", time.Time{}, err
	}
	return raw.HistoryID, time.UnixMilli(raw.Expiration).UTC(), nil
}

// SendAs lists the verified addresses the mailbox sends as, its own included.
func (c *Client) SendAs(ctx context.Context) ([]string, error) {
	var raw struct {
		SendAs []struct{ SendAsEmail, VerificationStatus string }
	}
	if err := c.get(ctx, c.gmail("settings/sendAs"), nil, &raw); err != nil {
		return nil, err
	}
	addresses := []string{}
	for _, s := range raw.SendAs {
		if s.VerificationStatus == "" || s.VerificationStatus == "accepted" {
			addresses = append(addresses, strings.ToLower(s.SendAsEmail))
		}
	}
	return addresses, nil
}

func (c *Client) gmail(path string) string {
	return c.endpoints.Gmail + "/gmail/v1/users/me/" + path
}

// addresses falls back to splitting on commas and angle brackets because Gmail
// decodes encoded-word display names without re-quoting them.
func addresses(header string) []Address {
	list, err := mail.ParseAddressList(header)
	if err != nil {
		for entry := range strings.SplitSeq(header, ",") {
			name, addr, found := strings.Cut(entry, "<")
			if !found {
				name, addr = "", name
			}
			addr, _, _ = strings.Cut(addr, ">")
			if strings.Contains(addr, "@") {
				list = append(list, &mail.Address{Name: name, Address: strings.TrimSpace(addr)})
			}
		}
	}
	var out []Address
	for _, a := range list {
		out = append(out, Address{Name: strings.Trim(a.Name, ` "'`), Email: strings.ToLower(a.Address)})
	}
	return out
}

func messageIDs(header string) []string {
	return strings.FieldsFunc(header, func(r rune) bool { return strings.ContainsRune("<>, \t\r\n", r) })
}

func firstID(header string) string {
	ids := messageIDs(header)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func bulk(h map[string]string) bool {
	precedence := strings.ToLower(h["precedence"])
	auto := strings.ToLower(h["auto-submitted"])
	return h["list-unsubscribe"] != "" ||
		precedence == "bulk" || precedence == "list" || precedence == "junk" ||
		auto != "" && auto != "no"
}
