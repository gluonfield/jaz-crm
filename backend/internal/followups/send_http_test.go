package followups_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/authapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-tasks/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func releaseThroughHTTP(t *testing.T, keys *auth.Service, services mcpapi.Services, userID, id string, seen followups.Seen) {
	t.Helper()
	logger := log.New(io.Discard)
	authn, err := authapi.NewHandler(keys, services.Workspaces, signin.Config{PublicURL: keys.Issuer()}, logger)
	if err != nil {
		t.Fatal(err)
	}
	handler := mcpapi.NewHandler(services, keys, logger)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler.MCP)
	mux.Handle("POST /api/tools/{tool}", authn.Session(handler.API))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	key, _, err := keys.CreateKey(ctx, userID, "embedded app", "")
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"record_id": id, "draft": seen.Draft, "from": seen.From, "to": seen.To, "cc": seen.Cc}
	client := mcp.NewClient(&mcp.Implementation{Name: "embedded-app", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: sendBearer(key)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "send_draft", Arguments: input})
	if err != nil || !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "only a person") {
		t.Fatalf("MCP must retain the human approval boundary: %v %v", result, err)
	}
	token, _, err := keys.CreateSession(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	post := func(cookie bool) (int, string) {
		t.Helper()
		body, _ := json.Marshal(input)
		request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/tools/send_draft", strings.NewReader(string(body)))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+key)
		if cookie {
			request.AddCookie(&http.Cookie{Name: "jc_session", Value: token})
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(response.Body)
		return response.StatusCode, string(raw)
	}
	if status, body := post(false); status != http.StatusUnauthorized {
		t.Fatalf("a bearer token became browser approval: %d %s", status, body)
	}
	input["draft"] = "A stale preview"
	if status, body := post(true); status != http.StatusBadRequest || !strings.Contains(body, "draft changed") {
		t.Fatalf("a changed draft was sent: %d %s", status, body)
	}
	input["draft"] = seen.Draft
	if status, body := post(true); status != http.StatusOK || !strings.Contains(body, `"draft_status":"Sent"`) {
		t.Fatalf("the browser confirmation could not send: %d %s", status, body)
	}
	if status, body := post(true); status != http.StatusBadRequest {
		t.Fatalf("a second confirmation sent twice: %d %s", status, body)
	}
}

type sendBearer string

func (token sendBearer) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+string(token))
	return http.DefaultTransport.RoundTrip(request)
}
