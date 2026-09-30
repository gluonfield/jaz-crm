package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"go.temporal.io/sdk/testsuite"
)

var ctx = context.Background()

type started []string

func (s *started) Start(_ context.Context, id string) error {
	*s = append(*s, id)
	return nil
}

func (s *started) Stop(context.Context, string) error { return nil }

func (s *started) Step(context.Context, string) (string, error) { return "", nil }

// fakeGoogle serves the mailbox, calendar and Meet of owner@cas.dev.
type fakeGoogle struct {
	noContacts bool
	messages   map[string]string
	listed     []string
	history    map[string]string
	now        time.Time
}

func gmailMessage(id, thread, from, to, subject, inReplyTo, body string, at time.Time) string {
	headers := []map[string]string{{"name": "From", "value": from}, {"name": "To", "value": to}, {"name": "Subject", "value": subject}, {"name": "Message-ID", "value": "<" + id + "@mail>"}}
	if inReplyTo != "" {
		headers = append(headers, map[string]string{"name": "In-Reply-To", "value": "<" + inReplyTo + "@mail>"})
	}
	raw, _ := json.Marshal(map[string]any{
		"id": id, "threadId": thread, "labelIds": []string{"INBOX"}, "internalDate": fmt.Sprint(at.UnixMilli()),
		"payload": map[string]any{"mimeType": "text/plain", "headers": headers, "body": map[string]string{"data": base64.RawURLEncoding.EncodeToString([]byte(body))}},
	})
	return string(raw)
}

func (f *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path
	switch {
	case path == "/token":
		fmt.Fprint(w, `{"access_token":"at","token_type":"Bearer","expires_in":3600,"refresh_token":"rt"}`)
	case path == "/gmail/v1/users/me/profile":
		fmt.Fprint(w, `{"emailAddress":"owner@cas.dev","historyId":"100"}`)
	case path == "/v1/otherContacts" && f.noContacts:
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"code":403,"status":"PERMISSION_DENIED","details":[{"reason":"ACCESS_TOKEN_SCOPE_INSUFFICIENT"}]}}`)
	case path == "/v1/otherContacts" && r.URL.Query().Get("pageToken") == "":
		fmt.Fprint(w, `{"otherContacts":[{"emailAddresses":[{"value":"Ada@Customer.io"}],"photos":[{"url":"https://lh3.googleusercontent.com/ada"}]}],"nextPageToken":"p2"}`)
	case path == "/v1/otherContacts":
		fmt.Fprint(w, `{"otherContacts":[{"emailAddresses":[{"value":"bob@supplier.com"}],"photos":[{"url":"https://lh3.googleusercontent.com/letter-b","default":true}]}]}`)
	case path == "/gmail/v1/users/me/settings/sendAs":
		fmt.Fprint(w, `{"sendAs":[{"sendAsEmail":"owner@cas.dev","isPrimary":true},{"sendAsEmail":"Sales@CAS-Alias.dev","verificationStatus":"accepted"},{"sendAsEmail":"unverified@elsewhere.dev","verificationStatus":"pending"}]}`)
	case path == "/gmail/v1/users/me/messages":
		var list []map[string]string
		for _, id := range f.listed {
			list = append(list, map[string]string{"id": id})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"messages": list})
	case strings.HasPrefix(path, "/gmail/v1/users/me/messages/"):
		fmt.Fprint(w, f.messages[strings.TrimPrefix(path, "/gmail/v1/users/me/messages/")])
	case path == "/gmail/v1/users/me/history":
		reply, ok := f.history[r.URL.Query().Get("startHistoryId")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"code":404,"message":"Requested entity was not found."}}`)
			return
		}
		fmt.Fprint(w, reply)
	case path == "/calendar/v3/calendars/primary/events":
		_ = json.NewEncoder(w).Encode(map[string]any{"nextSyncToken": "s1", "items": []any{
			map[string]any{"id": "ev0", "status": "cancelled"},
			map[string]any{
				"id": "ev1", "status": "confirmed", "summary": "Line review",
				"start": map[string]any{"dateTime": f.now.Add(-2 * time.Hour)}, "end": map[string]any{"dateTime": f.now.Add(-time.Hour)},
				"attendees": []any{
					map[string]any{"email": "owner@cas.dev", "self": true, "organizer": true, "responseStatus": "accepted"},
					map[string]any{"email": "ada@customer.io", "displayName": "Ada", "responseStatus": "accepted"},
					map[string]any{"email": "press-room@resource.calendar.google.com", "resource": true},
				},
				"conferenceData": map[string]any{"conferenceId": "abc-defg-hij", "conferenceSolution": map[string]any{"key": map[string]any{"type": "hangoutsMeet"}}},
			},
		}})
	case path == "/v2/conferenceRecords":
		_ = json.NewEncoder(w).Encode(map[string]any{"conferenceRecords": []any{
			map[string]any{"name": "conferenceRecords/old", "startTime": f.now.Add(-8 * 24 * time.Hour)},
			map[string]any{"name": "conferenceRecords/c1", "startTime": f.now.Add(-2 * time.Hour)},
		}})
	case path == "/v2/conferenceRecords/c1/transcripts":
		fmt.Fprint(w, `{"transcripts":[{"name":"conferenceRecords/c1/transcripts/t1","state":"FILE_GENERATED"}]}`)
	case path == "/v2/conferenceRecords/c1/participants":
		fmt.Fprint(w, `{"participants":[{"name":"conferenceRecords/c1/participants/p1","signedinUser":{"displayName":"Ada"}}]}`)
	case path == "/v2/conferenceRecords/c1/transcripts/t1/entries":
		_ = json.NewEncoder(w).Encode(map[string]any{"transcriptEntries": []any{
			map[string]any{"name": "conferenceRecords/c1/transcripts/t1/entries/e1", "participant": "conferenceRecords/c1/participants/p1", "text": "Can you do 200 a week?", "startTime": f.now.Add(-2 * time.Hour)},
		}})
	default:
		http.Error(w, "unexpected "+path, http.StatusTeapot)
	}
}

// One connection syncs end to end against a fake Google: consent, mail
// backfill, triage, bodies for kept threads only, incremental mail, an
// expired history cursor, meetings and a Meet transcript.
func TestSyncAgainstGoogle(t *testing.T) {
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@cas.dev")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	now := time.Now().UTC()
	fake := &fakeGoogle{now: now, messages: map[string]string{
		"m1": gmailMessage("m1", "t1", "Owner <owner@cas.dev>", "Ada Lovelace <ada@customer.io>", "Quote", "", "Hello Ada", now.Add(-48*time.Hour)),
		"m2": gmailMessage("m2", "t1", "Ada Lovelace <ada@customer.io>", "owner@cas.dev", "Re: Quote", "m1", "Thanks!", now.Add(-47*time.Hour)),
		"m4": gmailMessage("m4", "t2", "bob@supplier.com", "owner@cas.dev", "Cold pitch", "", "Buy our bolts", now.Add(-40*time.Hour)),
		"m3": gmailMessage("m3", "t3", "ada@customer.io", "owner@cas.dev", "Next order", "", "Another 50 please", now.Add(-time.Hour)),
	}, listed: []string{"m1", "m2", "m4"}, history: map[string]string{
		"100": `{"history":[{"messagesAdded":[{"message":{"id":"m3"}}]}],"historyId":"120"}`,
	}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	var starts started
	conns, err := connections.NewService(store, connections.Config{
		Google:    google.OAuthConfig{ClientID: "client", ClientSecret: "secret", TokenURL: srv.URL + "/token"},
		Key:       []byte("0123456789abcdef0123456789abcdef"),
		Endpoints: google.Endpoints{Gmail: srv.URL, Calendar: srv.URL, Meet: srv.URL, People: srv.URL},
		Backfill:  365 * 24 * time.Hour,
	}, &starts)
	if err != nil {
		t.Fatal(err)
	}
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	conn, err := conns.Connect(ctx, actor, "code", "verifier")
	if err != nil || conn.Account != "owner@cas.dev" || len(starts) != 1 {
		t.Fatalf("connect: %+v %v %v", conn, starts, err)
	}
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	a := NewActivities(conns, convs, Config{})
	env.RegisterActivity(a)
	run := func(activity any, out any, args ...any) {
		t.Helper()
		value, err := env.ExecuteActivity(activity, args...)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			if err := value.Get(out); err != nil {
				t.Fatal(err)
			}
		}
	}
	run(a.Aliases, nil, conn.ID)
	if own, err := store.InternalAddresses(ctx, owner.WorkspaceID); err != nil || !slices.Contains(own, "sales@cas-alias.dev") || slices.Contains(own, "unverified@elsewhere.dev") {
		t.Fatalf("own addresses: %v %v", own, err)
	}
	var done bool
	run(a.GmailBackfill, &done, conn.ID)
	run(a.Triage, nil, conn.ID)
	var fetched int
	run(a.FetchContent, &fetched, conn.ID)
	if !done || fetched != 2 {
		t.Fatalf("backfill done %v, fetched %d bodies; want only the kept thread's two", done, fetched)
	}
	if views, err := conns.List(ctx, actor); err != nil || len(views) != 1 || !views[0].Backfilled || views[0].Messages != 3 || !views[0].Oldest.Equal(now.Add(-48*time.Hour).Truncate(time.Millisecond)) {
		t.Fatalf("progress: %+v %v", views, err)
	}
	kept, err := convs.Contacts(ctx, actor, interactions.Kept, "", 10)
	if err != nil || len(kept) != 1 || kept[0].Address != "ada@customer.io" {
		t.Fatalf("kept: %+v %v", kept, err)
	}
	ada := kept[0].PersonID

	fake.noContacts = true
	run(a.Photos, nil, conn.ID)
	if cursor, _ := conns.Cursor(ctx, conn.ID, connections.StreamPhotos); cursor != "" {
		t.Fatalf("an account without contacts access must retry after reconnecting: %q", cursor)
	}
	fake.noContacts = false
	run(a.Photos, nil, conn.ID)
	photos, err := convs.Photos(ctx, actor, []string{ada})
	pending, _ := convs.Contacts(ctx, actor, interactions.Pending, "", 10)
	if err != nil || photos[ada] != "https://lh3.googleusercontent.com/ada" || len(pending) != 1 || pending[0].Photo != "" {
		t.Fatalf("photos: %v %+v %v", photos, pending, err)
	}
	thread, err := convs.Timeline(ctx, actor, ada, nil, nil, 10)
	if err != nil || len(thread) != 1 || thread[0].Preview != "Hello Ada" || len(thread[0].Participants) != 2 {
		t.Fatalf("thread: %+v %v", thread, err)
	}

	run(a.GmailIncremental, nil, conn.ID)
	run(a.FetchContent, &fetched, conn.ID)
	if list, _ := convs.Timeline(ctx, actor, ada, nil, nil, 10); len(list) != 2 || list[0].Title != "Next order" || fetched != 1 {
		t.Fatalf("incremental: %+v, fetched %d", list, fetched)
	}
	if cursor, _ := conns.Cursor(ctx, conn.ID, connections.StreamHistory); cursor != "120" {
		t.Fatalf("history cursor: %q", cursor)
	}
	run(a.GmailIncremental, nil, conn.ID)
	if backfill, _ := conns.Cursor(ctx, conn.ID, connections.StreamBackfill); backfill != "" {
		t.Fatalf("an expired history cursor must restart the backfill: %q", backfill)
	}

	run(a.CalendarSync, nil, conn.ID)
	var due []MeetingRef
	run(a.DueMeetings, &due, conn.ID)
	if len(due) != 1 || due[0].MeetCode != "abc-defg-hij" {
		t.Fatalf("due meetings: %+v", due)
	}
	var found bool
	run(a.FetchTranscript, &found, due[0])
	meeting, err := convs.Get(ctx, actor, due[0].InteractionID)
	if err != nil || !found || len(meeting.Parts) != 1 || meeting.Parts[0].Author != "Ada" || len(meeting.Participants) != 2 || meeting.Participants[1].Role != "organizer" {
		t.Fatalf("meeting: %+v %v", meeting, err)
	}
}
