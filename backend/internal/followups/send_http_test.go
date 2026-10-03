package followups_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func releaseThroughHTTP(t *testing.T, keys *auth.Service, services mcpapi.Services, userID, id string, seen followups.Seen, transport string) {
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
	input := map[string]any{"confirmed": false, "record_id": id, "draft": seen.Draft, "subject": seen.Subject, "from": seen.From, "to": seen.To, "cc": seen.Cc}
	client := mcp.NewClient(&mcp.Implementation{Name: "embedded-app", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: sendBearer(key)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "send_draft", Arguments: input})
	if err != nil || !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "confirm the reply") {
		t.Fatalf("MCP must require confirmation: %v %v", result, err)
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
	if status, body := post(true); status != http.StatusBadRequest || !strings.Contains(body, "confirm the reply") {
		t.Fatalf("browser must require confirmation: %d %s", status, body)
	}
	save := map[string]any{"record_id": id, "draft": seen.Draft + " Edited in the browser.", "channel": "Email", "subject": seen.Subject, "to": []string{"old@example.com"}, "cc": []string{"old-copy@example.com"}}
	body, _ := json.Marshal(save)
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/tools/save_draft", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "jc_session", Value: token})
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("browser composer save: %d", response.StatusCode)
	}
	save["draft"] = seen.Draft
	save["to"] = seen.To
	save["cc"] = seen.Cc
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "save_draft", Arguments: save})
	if err != nil || result.IsError {
		t.Fatalf("the embedded composer could not edit the browser's draft: %v %v", result, err)
	}
	var saved struct {
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &saved); err != nil || saved.Values["draft"] != seen.Draft || saved.Values["draft_status"] != "Draft" {
		t.Fatalf("the embedded edit was skipped or retained approval: %+v %v", saved, err)
	}
	for attribute, addresses := range map[string][]string{"to": seen.To, "cc": seen.Cc} {
		raw, _ := json.Marshal(addresses)
		var expected any
		_ = json.Unmarshal(raw, &expected)
		if !reflect.DeepEqual(saved.Values[attribute], expected) {
			t.Fatalf("%s retained old recipients: %+v", attribute, saved.Values[attribute])
		}
	}
	cleared, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "save_draft", Arguments: map[string]any{
		"record_id": id, "draft": "", "channel": "Email", "subject": seen.Subject, "to": []string{}, "cc": []string{},
	}})
	if err != nil || cleared.IsError {
		t.Fatalf("clear draft: %v %v", cleared, err)
	}
	saved.Values = nil
	if err := json.Unmarshal([]byte(cleared.Content[0].(*mcp.TextContent).Text), &saved); err != nil || saved.Values["draft"] != nil || saved.Values["draft_status"] != nil || saved.Values["to"] != nil || saved.Values["cc"] != nil {
		t.Fatalf("clearing the composer retained draft data: %+v %v", saved, err)
	}
	if restored, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "save_draft", Arguments: save}); err != nil || restored.IsError {
		t.Fatalf("restore draft: %v %v", restored, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "upsert_record", Arguments: map[string]any{
		"object": "follow_ups", "record_id": id, "values": map[string]string{"draft_status": "Approved"},
	}})
	if err != nil || !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "only a person") {
		t.Fatalf("an ordinary record write bypassed confirmation: %v %v", result, err)
	}
	release := func() (bool, string) {
		t.Helper()
		if transport == "browser" {
			status, body := post(true)
			return status == http.StatusOK, body
		}
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "send_draft", Arguments: input})
		if err != nil {
			t.Fatal(err)
		}
		return !result.IsError, result.Content[0].(*mcp.TextContent).Text
	}
	input["confirmed"] = true
	input["draft"] = "A stale preview"
	if success, body := release(); success || !strings.Contains(body, "draft changed") {
		t.Fatalf("a changed draft was sent: %v %s", success, body)
	}
	input["draft"] = seen.Draft
	input["subject"] = "An unreviewed subject"
	if success, body := release(); success || !strings.Contains(body, "subject changed") {
		t.Fatalf("a changed subject was sent: %v %s", success, body)
	}
	input["subject"] = seen.Subject
	if success, body := release(); !success || !strings.Contains(body, `"draft_status":"Sent"`) {
		t.Fatalf("the confirmation could not send: %v %s", success, body)
	}
	if success, body := release(); success {
		t.Fatalf("a second confirmation sent twice: %s", body)
	}
}

type sendBearer string

func (token sendBearer) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+string(token))
	return http.DefaultTransport.RoundTrip(request)
}
