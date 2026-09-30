package google

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Attendee struct {
	Email                     string
	Name                      string `json:"displayName"`
	Response                  string `json:"responseStatus"`
	Self, Organizer, Resource bool
}

type Event struct {
	ID, ICalUID, Status, Summary, Description string
	Start, End                                time.Time
	AllDay                                    bool
	Organizer                                 Attendee
	Attendees                                 []Attendee
	MeetCode                                  string
	Recurring                                 bool
}

type EventPage struct {
	Events          []Event
	Next, SyncToken string
}

// apiEvent decodes the fields Event shares with the wire format straight into it.
type apiEvent struct {
	Event
	Start, End       eventTime
	HangoutLink      string
	RecurringEventID string
	ConferenceData   struct {
		ConferenceID       string
		ConferenceSolution struct{ Key struct{ Type string } }
	}
}

type eventTime struct {
	Date     string
	DateTime time.Time
}

func (c *Client) Events(ctx context.Context, syncToken, pageToken string, timeMin time.Time) (EventPage, error) {
	q := url.Values{"singleEvents": {"true"}, "showDeleted": {"true"}, "maxResults": {"250"}, "pageToken": {pageToken}, "syncToken": {syncToken}}
	if syncToken == "" {
		q.Set("timeMin", timeMin.Format(time.RFC3339))
	}
	var raw struct {
		Items                        []apiEvent
		NextPageToken, NextSyncToken string
	}
	if err := c.get(ctx, c.calendar("calendars/primary/events"), q, &raw); err != nil {
		return EventPage{}, err
	}
	page := EventPage{Next: raw.NextPageToken, SyncToken: raw.NextSyncToken}
	for _, item := range raw.Items {
		page.Events = append(page.Events, item.event())
	}
	return page, nil
}

func (c *Client) WatchEvents(ctx context.Context, channelID, address, token string, ttl time.Duration) (resourceID string, expiration time.Time, err error) {
	body := map[string]any{
		"id":      channelID,
		"type":    "web_hook",
		"address": address,
		"token":   token,
		"params":  map[string]string{"ttl": strconv.Itoa(int(ttl.Seconds()))},
	}
	var raw struct {
		ResourceID string
		Expiration int64 `json:",string"`
	}
	if err := c.post(ctx, c.calendar("calendars/primary/events/watch"), body, &raw); err != nil {
		return "", time.Time{}, err
	}
	return raw.ResourceID, time.UnixMilli(raw.Expiration).UTC(), nil
}

func (c *Client) StopChannel(ctx context.Context, channelID, resourceID string) error {
	return c.post(ctx, c.calendar("channels/stop"), map[string]string{"id": channelID, "resourceId": resourceID}, nil)
}

func (c *Client) calendar(path string) string {
	return c.endpoints.Calendar + "/calendar/v3/" + path
}

func (raw apiEvent) event() Event {
	e := raw.Event
	e.Start = raw.Start.time()
	e.End = raw.End.time()
	e.AllDay = raw.Start.Date != ""
	e.Organizer.Email = strings.ToLower(e.Organizer.Email)
	for i := range e.Attendees {
		e.Attendees[i].Email = strings.ToLower(e.Attendees[i].Email)
	}
	e.MeetCode = raw.meetCode()
	e.Recurring = raw.RecurringEventID != ""
	return e
}

func (raw apiEvent) meetCode() string {
	if raw.ConferenceData.ConferenceSolution.Key.Type == "hangoutsMeet" {
		return raw.ConferenceData.ConferenceID
	}
	link, err := url.Parse(raw.HangoutLink)
	if err != nil {
		return ""
	}
	return strings.Trim(link.Path, "/")
}

func (t eventTime) time() time.Time {
	if t.Date == "" {
		return t.DateTime
	}
	date, _ := time.Parse(time.DateOnly, t.Date)
	return date
}
