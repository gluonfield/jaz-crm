package authapi_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/gluonfield/jaz-tasks/auth"
)

func TestPageImageUploadAndReferences(t *testing.T) {
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
	pixels := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	pixels.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 40, A: 90})
	var source bytes.Buffer
	if err := png.Encode(&source, pixels); err != nil {
		t.Fatal(err)
	}
	upload := "data:image/png;base64," + base64.StdEncoding.EncodeToString(source.Bytes())
	page := invoke("upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Design", "content": "Keep the drawing", "icon": upload}})["record"].(map[string]any)
	id := page["id"].(string)
	icon := page["values"].(map[string]any)["icon"].(string)
	if !strings.HasPrefix(icon, "image:"+s.url+"/page-icons/") || strings.Contains(icon, "base64") {
		t.Fatalf("an image is a small URL in tool output: %q", icon)
	}
	checkImage := func(icon string, status int) {
		t.Helper()
		res, err := http.Get(strings.TrimPrefix(icon, "image:"))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != status {
			t.Fatalf("image response: %d, want %d", res.StatusCode, status)
		}
		if status != http.StatusOK {
			return
		}
		decoded, err := png.Decode(res.Body)
		if err != nil || decoded.Bounds() != pixels.Bounds() || color.NRGBAModel.Convert(decoded.At(0, 0)) != pixels.NRGBAAt(0, 0) || res.Header.Get("Content-Type") != "image/png" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("image pixels, transparency or headers: %v %v", err, res.Header)
		}
	}
	checkImage(icon, http.StatusOK)
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
	same := invoke("upsert_record", map[string]any{"object": "pages", "record_id": id, "values": map[string]any{"icon": icon}})["record"].(map[string]any)
	if same["values"].(map[string]any)["icon"] != icon {
		t.Fatal("saving the same icon creates another image")
	}
	child := invoke("upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Child", "parent": id}})["record"].(map[string]any)["id"].(string)
	a := s.agent(t, b)
	var got map[string]any
	if err := json.Unmarshal([]byte(callText(t, a, "get_record", map[string]any{"record_id": child})), &got); err != nil {
		t.Fatal(err)
	}
	if got["values"].(map[string]any)["parent"].(map[string]any)["icon"] != icon {
		t.Fatalf("MCP reference loses the image: %v", got)
	}
	copy := invoke("upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Copy", "icon": icon}})["record"].(map[string]any)["values"].(map[string]any)["icon"].(string)
	if copy == icon {
		t.Fatal("a copied image depends on the original page's lifetime")
	}
	for _, value := range []string{"📚", upload, "icon:rocket"} {
		previous := icon
		invoke("upsert_record", map[string]any{"object": "pages", "record_id": id, "values": map[string]any{"icon": value}, "expect": map[string]any{"icon": icon}})
		icon = invoke("get_record", map[string]any{"record_id": id})["values"].(map[string]any)["icon"].(string)
		if value == upload {
			if icon == previous || !strings.HasPrefix(icon, "image:"+s.url+"/page-icons/") {
				t.Fatalf("image replacement: %q", icon)
			}
			checkImage(icon, http.StatusOK)
		} else if icon != value {
			t.Fatalf("icon replacement: %q, want %q", icon, value)
		}
	}
	invoke("upsert_record", map[string]any{"object": "pages", "record_id": id, "values": map[string]any{"icon": upload}})
	icon = invoke("get_record", map[string]any{"record_id": id})["values"].(map[string]any)["icon"].(string)
	cleared := invoke("upsert_record", map[string]any{"object": "pages", "record_id": id, "remove": map[string]any{"icon": []string{icon}}})["record"].(map[string]any)["values"].(map[string]any)
	if cleared["icon"] != nil || cleared["content"] != "Keep the drawing" {
		t.Fatalf("image removal loses content: %v", cleared)
	}
	checkImage(icon, http.StatusOK)
	invoke("delete_record", map[string]any{"record_id": id})
	checkImage(icon, http.StatusNotFound)
	checkImage(copy, http.StatusOK)
	checkImage("image:"+s.url+"/page-icons/"+child, http.StatusNotFound)
}
