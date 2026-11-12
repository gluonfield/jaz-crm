package worker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/interactions"
	"github.com/gluonfield/jaz-crm/internal/connections"
	"github.com/gluonfield/jaz-crm/internal/google"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/internal/workspaces"
	"github.com/gluonfield/jaz-crm/records"
)

func TestCalendarWatchRegistersAndRenewsValidChannels(t *testing.T) {
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(t.Context(), "owner@example.test")
	if err != nil {
		t.Fatal(err)
	}
	var registered []string
	var stopped string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			fmt.Fprint(w, `{"access_token":"access","token_type":"Bearer","expires_in":3600,"refresh_token":"refresh"}`)
		case "/gmail/v1/users/me/profile":
			fmt.Fprint(w, `{"emailAddress":"owner@example.test","historyId":"1"}`)
		case "/calendar/v3/calendars/primary/events/watch":
			var channel struct{ ID, Type, Address, Token string }
			if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
				t.Error(err)
			}
			if !regexp.MustCompile(`^[A-Za-z0-9_+/=-]{1,64}$`).MatchString(channel.ID) || channel.Type != "web_hook" || channel.Token == "" || channel.Address != "https://crm.example.test/webhooks/calendar" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":{"message":"Invalid channel ID","errors":[{"reason":"channelIdInvalid"}]}}`)
				return
			}
			registered = append(registered, channel.ID)
			fmt.Fprintf(w, `{"resourceId":"resource","expiration":%q}`, fmt.Sprint(time.Now().Add(7*24*time.Hour).UnixMilli()))
		case "/calendar/v3/channels/stop":
			var channel struct{ ID, ResourceID string }
			if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
				t.Error(err)
			}
			stopped = channel.ID
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(api.Close)
	conns, err := connections.NewService(store, connections.Config{
		Google: google.OAuthConfig{ClientID: "client", ClientSecret: "secret", TokenURL: api.URL + "/token"},
		Key:    []byte("0123456789abcdef0123456789abcdef"), Endpoints: google.Endpoints{Gmail: api.URL, Calendar: api.URL},
	}, &started{})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := conns.Connect(t.Context(), records.Actor{MemberID: owner.ID, WorkspaceID: owner.WorkspaceID}, "code", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	a := &Activities{Connections: conns, Interactions: convs, Config: Config{CalendarWebhook: "https://crm.example.test/webhooks/calendar"}}
	for range 2 {
		if err := a.Watch(t.Context(), connection.ID); err != nil {
			t.Fatal(err)
		}
	}
	channel, err := conns.Channel(t.Context(), connection.ID)
	if err != nil || len(registered) != 1 || channel.ID != registered[0] || channel.Resource != "resource" {
		t.Fatalf("channel was not persisted or was registered repeatedly: %+v, %v (%v)", channel, registered, err)
	}
	channel.Expires = time.Now()
	if err := conns.SetChannel(t.Context(), connection.ID, channel); err != nil {
		t.Fatal(err)
	}
	if err := a.Watch(t.Context(), connection.ID); err != nil {
		t.Fatal(err)
	}
	if len(registered) != 2 || registered[0] == registered[1] || stopped != registered[0] {
		t.Fatalf("renewal reused identity or failed to stop old channel: registered=%v stopped=%q", registered, stopped)
	}
}
