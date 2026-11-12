package mcpapi_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/internal/status"
	"github.com/gluonfield/jaz-crm/internal/storage"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres"
	signin "github.com/gluonfield/jaz-tasks/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOperationErrorsRespectMemberAndWorkspace(t *testing.T) {
	ctx := t.Context()
	e := serve(t)
	store := e.store.(*postgres.Store)
	key := e.apiKey(t, "owner@example.test")
	owner := e.session(t, key)
	actor, err := e.keys.Authenticate(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	mustCall(t, owner, "invite_member", map[string]any{"email": "member@example.test"})
	member, err := e.people.SignIn(ctx, signin.Identity{Issuer: "test", Subject: "member", Email: "member@example.test", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	memberKey, _, err := e.keys.CreateKey(ctx, member.ID, "test", "")
	if err != nil {
		t.Fatal(err)
	}
	otherKey := e.apiKey(t, "other@example.test")
	other, err := e.keys.Authenticate(ctx, otherKey)
	if err != nil {
		t.Fatal(err)
	}
	writer := status.NewService(store)
	now := time.Now()
	ids := map[string]string{}
	for _, user := range []struct{ workspace, user, account string }{
		{actor.WorkspaceID, actor.MemberID, "own-mail@example.test"},
		{member.WorkspaceID, member.ID, "private-member@example.test"},
		{other.WorkspaceID, other.MemberID, "private-other@example.test"},
	} {
		connection, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: user.workspace, UserID: user.user, Provider: "google", Account: user.account, RefreshToken: []byte("encrypted")})
		if err != nil {
			t.Fatal(err)
		}
		ids[user.user] = "GmailDrafts:" + connection.ID
		if err := writer.Observe(ctx, "GmailDrafts", &connection.ID, status.Observation{ObservedAt: now, OccurredAt: &now, Reason: "Sync could not finish."}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Observe(ctx, status.AI, nil, status.Observation{ObservedAt: now, OccurredAt: &now, RetryAt: new(now.Add(time.Hour)), Reason: "Check the model provider's credit or usage limit."}); err != nil {
		t.Fatal(err)
	}
	for _, caller := range []struct {
		user    string
		session *mcp.ClientSession
	}{{actor.MemberID, owner}, {member.ID, e.session(t, memberKey)}, {other.MemberID, e.session(t, otherKey)}} {
		out := mustCall(t, caller.session, "get_operation_errors", nil)
		rows := out["errors"].([]any)
		if len(rows) != 2 {
			t.Fatalf("status leaked another member's connection or missed shared AI issue: %s", encode(out))
		}
		for _, row := range rows {
			id := row.(map[string]any)["id"]
			if id != status.AI && id != ids[caller.user] {
				t.Fatalf("status crossed member/workspace boundary: %s", encode(out))
			}
		}
		encoded := encode(out)
		if !strings.Contains(encoded, "AI background work") || !strings.Contains(encoded, "Gmail drafts") || strings.Contains(encoded, "@example.test") || strings.Contains(encoded, "encrypted") {
			t.Fatalf("status exposed account or provider details: %s", encoded)
		}
	}
}
