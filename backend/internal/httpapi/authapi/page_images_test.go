package authapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/gluonfield/jaz-tasks/auth"
)

func TestPageIconLinksColorsAndReferences(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})
	b := browser()
	s.ownerSession(t, b)
	invoke := func(tool string, args map[string]any) map[string]any {
		t.Helper()
		raw, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		status, body := session(t, s, b, http.MethodPost, "/api/tools/"+tool, string(raw))
		var out map[string]any
		if status != http.StatusOK || json.Unmarshal([]byte(body), &out) != nil {
			t.Fatalf("%s: %d %s", tool, status, body)
		}
		return out
	}
	var fetches atomic.Int64
	image := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte("<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 16 16\"><circle cx=\"8\" cy=\"8\" r=\"8\"/></svg>"))
	}))
	t.Cleanup(image.Close)
	icon := "image:" + image.URL + "/logo.svg?size=128&v=1"
	page := invoke("upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Design", "content": "Keep the drawing", "icon": icon}})["record"].(map[string]any)
	id := page["id"].(string)
	if got := page["values"].(map[string]any)["icon"]; got != icon {
		t.Fatalf("image URL changed in tool output: %v", got)
	}
	history := invoke("record_history", map[string]any{"record_id": id})["changes"].([]any)
	found := false
	for _, raw := range history {
		change := raw.(map[string]any)
		if change["attribute"] == "icon" && change["value"] == icon {
			found = true
		}
	}
	if !found {
		t.Fatal("history loses the image URL")
	}
	child := invoke("upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Child", "parent": id}})["record"].(map[string]any)["id"].(string)
	a := s.agent(t, b)
	for _, value := range []string{icon, "icon:Drill:blue", "icon:Drill:red", "📚"} {
		invoke("upsert_record", map[string]any{"object": "pages", "record_id": id, "values": map[string]any{"icon": value}, "expect": map[string]any{"icon": icon}})
		icon = invoke("get_record", map[string]any{"record_id": id})["values"].(map[string]any)["icon"].(string)
		if icon != value {
			t.Fatalf("icon replacement: %q, want %q", icon, value)
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(callText(t, a, "get_record", map[string]any{"record_id": child})), &got); err != nil {
			t.Fatal(err)
		}
		if got["values"].(map[string]any)["parent"].(map[string]any)["icon"] != icon {
			t.Fatalf("MCP reference loses the icon: %v", got)
		}
	}
	status, body := session(t, s, b, http.MethodPost, "/api/tools/upsert_record", "{\"object\":\"pages\",\"record_id\":\""+id+"\",\"values\":{\"icon\":\"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB\"}}")
	if status != http.StatusBadRequest || !strings.Contains(body, "file uploads are unavailable") {
		t.Fatalf("file upload accepted: %d %s", status, body)
	}
	if got := invoke("get_record", map[string]any{"record_id": id})["values"].(map[string]any)["icon"]; got != icon {
		t.Fatalf("rejected upload replaced the icon: %v", got)
	}
	cleared := invoke("upsert_record", map[string]any{"object": "pages", "record_id": id, "remove": map[string]any{"icon": []string{icon}}})["record"].(map[string]any)["values"].(map[string]any)
	if cleared["icon"] != nil || cleared["content"] != "Keep the drawing" {
		t.Fatalf("icon removal loses content: %v", cleared)
	}
	if fetches.Load() != 0 {
		t.Fatal("saving an image URL downloaded it on the server")
	}
	res, err := http.Get(s.url + "/page-icons/" + child)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown stored image: %d", res.StatusCode)
	}
}
