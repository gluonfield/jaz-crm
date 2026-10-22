package google

import (
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestMeet(t *testing.T) {
	c := fake(t, map[string]http.HandlerFunc{
		"/v2/conferenceRecords": func(w http.ResponseWriter, r *http.Request) {
			wantQuery(t, r.URL, url.Values{"filter": {`space.meeting_code = "abc-mnop-xyz"`}})
			io.WriteString(w, `{"conferenceRecords":[{"name":"conferenceRecords/c1","startTime":"2026-02-02T10:01:00Z","endTime":"2026-02-02T10:29:00Z","space":"spaces/s1"}]}`)
		},
		"/v2/conferenceRecords/c1/transcripts/t1/entries": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("pageToken") == "" {
				wantQuery(t, r.URL, url.Values{"pageSize": {"100"}})
				io.WriteString(w, `{"nextPageToken":"n2","transcriptEntries":[{"name":"conferenceRecords/c1/transcripts/t1/entries/1","participant":"conferenceRecords/c1/participants/p1","text":"Hi","startTime":"2026-02-02T10:02:00Z","endTime":"2026-02-02T10:02:03Z"}]}`)
				return
			}
			wantQuery(t, r.URL, url.Values{"pageSize": {"100"}, "pageToken": {"n2"}})
			io.WriteString(w, `{"transcriptEntries":[{"name":"conferenceRecords/c1/transcripts/t1/entries/2","participant":"conferenceRecords/c1/participants/p2","text":"Hello"}]}`)
		},
		"/v2/conferenceRecords/c1/participants": respond(http.StatusOK, `{"participants":[
			{"name":"conferenceRecords/c1/participants/p1","signedinUser":{"user":"users/1","displayName":"Ann"}},
			{"name":"conferenceRecords/c1/participants/p2","anonymousUser":{"displayName":"Guest"}},
			{"name":"conferenceRecords/c1/participants/p3","phoneUser":{"displayName":"+1 555"}}]}`),
	})

	conferences, err := c.Conferences(t.Context(), "abc-mnop-xyz")
	if want := []Conference{{Name: "conferenceRecords/c1", Start: time.Date(2026, 2, 2, 10, 1, 0, 0, time.UTC), End: time.Date(2026, 2, 2, 10, 29, 0, 0, time.UTC)}}; err != nil || !reflect.DeepEqual(conferences, want) {
		t.Errorf("conferences = %+v, %v", conferences, err)
	}
	entries, err := c.Entries(t.Context(), "conferenceRecords/c1/transcripts/t1")
	want := []Entry{
		{Name: "conferenceRecords/c1/transcripts/t1/entries/1", Participant: "conferenceRecords/c1/participants/p1", Text: "Hi", Start: time.Date(2026, 2, 2, 10, 2, 0, 0, time.UTC), End: time.Date(2026, 2, 2, 10, 2, 3, 0, time.UTC)},
		{Name: "conferenceRecords/c1/transcripts/t1/entries/2", Participant: "conferenceRecords/c1/participants/p2", Text: "Hello"},
	}
	if err != nil || !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %+v, %v", entries, err)
	}
	participants, err := c.Participants(t.Context(), "conferenceRecords/c1")
	wantParticipants := []Participant{
		{Name: "conferenceRecords/c1/participants/p1", DisplayName: "Ann"},
		{Name: "conferenceRecords/c1/participants/p2", DisplayName: "Guest"},
		{Name: "conferenceRecords/c1/participants/p3", DisplayName: "+1 555"},
	}
	if err != nil || !reflect.DeepEqual(participants, wantParticipants) {
		t.Errorf("participants = %+v, %v", participants, err)
	}
}
