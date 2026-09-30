package google

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestEvents(t *testing.T) {
	timeMin := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := fake(t, map[string]http.HandlerFunc{
		"/calendar/v3/calendars/primary/events": func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Query().Get("pageToken") + r.URL.Query().Get("syncToken") {
			case "":
				wantQuery(t, r.URL, url.Values{"singleEvents": {"true"}, "showDeleted": {"true"}, "maxResults": {"250"}, "timeMin": {"2026-01-01T00:00:00Z"}})
				io.WriteString(w, `{"nextPageToken":"p2","items":[
					{"id":"e1","iCalUID":"e1@google.com","status":"confirmed","summary":"Sync","description":"Weekly","recurringEventId":"e0",
					 "start":{"dateTime":"2026-02-02T10:00:00Z"},"end":{"dateTime":"2026-02-02T10:30:00Z"},
					 "organizer":{"email":"Ann@X.com","displayName":"Ann","self":true},
					 "attendees":[{"email":"Ann@X.com","displayName":"Ann","responseStatus":"accepted","self":true,"organizer":true},
					              {"email":"room@resource.calendar.google.com","responseStatus":"needsAction","resource":true}],
					 "hangoutLink":"https://meet.google.com/zzz-zzzz-zzz",
					 "conferenceData":{"conferenceId":"abc-mnop-xyz","conferenceSolution":{"key":{"type":"hangoutsMeet"}}}},
					{"id":"e2","status":"tentative","start":{"date":"2026-02-03"},"end":{"date":"2026-02-04"},"hangoutLink":"https://meet.google.com/def-ghij-klm"},
					{"id":"e3","status":"cancelled"}]}`)
			case "p2":
				wantQuery(t, r.URL, url.Values{"singleEvents": {"true"}, "showDeleted": {"true"}, "maxResults": {"250"}, "timeMin": {"2026-01-01T00:00:00Z"}, "pageToken": {"p2"}})
				io.WriteString(w, `{"items":[],"nextSyncToken":"s1"}`)
			default:
				wantQuery(t, r.URL, url.Values{"singleEvents": {"true"}, "showDeleted": {"true"}, "maxResults": {"250"}, "syncToken": {"s1"}})
				respond(http.StatusGone, `{"error":{"code":410,"message":"Sync token is no longer valid, a full sync is required."}}`)(w, r)
			}
		},
	})

	page, err := c.Events(t.Context(), "", "", timeMin)
	if err != nil {
		t.Fatal(err)
	}
	want := EventPage{Next: "p2", Events: []Event{
		{
			ID:          "e1",
			ICalUID:     "e1@google.com",
			Status:      "confirmed",
			Summary:     "Sync",
			Description: "Weekly",
			Start:       time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC),
			End:         time.Date(2026, 2, 2, 10, 30, 0, 0, time.UTC),
			Organizer:   Attendee{Email: "ann@x.com", Name: "Ann", Self: true},
			Attendees: []Attendee{
				{Email: "ann@x.com", Name: "Ann", Response: "accepted", Self: true, Organizer: true},
				{Email: "room@resource.calendar.google.com", Response: "needsAction", Resource: true},
			},
			MeetCode:  "abc-mnop-xyz",
			Recurring: true,
		},
		{
			ID:       "e2",
			Status:   "tentative",
			Start:    time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC),
			End:      time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC),
			AllDay:   true,
			MeetCode: "def-ghij-klm",
		},
		{ID: "e3", Status: "cancelled"},
	}}
	if !reflect.DeepEqual(page, want) {
		t.Errorf("page =\n%+v\nwant\n%+v", page, want)
	}

	page, err = c.Events(t.Context(), "", "p2", timeMin)
	if err != nil || page.SyncToken != "s1" {
		t.Fatalf("second page = %+v, %v", page, err)
	}
	if _, err := c.Events(t.Context(), "s1", "", timeMin); !errors.Is(err, ErrExpiredCursor) {
		t.Errorf("err = %v, want ErrExpiredCursor", err)
	}
}
