package mcpapi_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestChatGPTContract(t *testing.T) {
	e := serve(t)
	key := e.apiKey(t, "owner@example.com")
	session := e.session(t, key)
	ctx := context.Background()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	additive := map[string]bool{"add_attribute_option": true, "create_object": true, "create_attribute": true, "create_workspace": true, "invite_member": true, "log_interaction": true}
	var profileTool *mcp.Tool
	for _, tool := range listed.Tools {
		annotations := tool.Annotations
		if annotations == nil || annotations.DestructiveHint == nil || annotations.OpenWorldHint == nil || *annotations.OpenWorldHint {
			t.Fatalf("%s has incomplete or open-world safety metadata: %+v", tool.Name, annotations)
		}
		if *annotations.DestructiveHint == (annotations.ReadOnlyHint || additive[tool.Name]) {
			t.Fatalf("%s has the wrong destructive classification: %+v", tool.Name, annotations)
		}
		schemes, _ := json.Marshal(tool.Meta["securitySchemes"])
		if string(schemes) != `[{"scopes":[],"type":"oauth2"}]` {
			t.Fatalf("%s does not declare workspace OAuth: %s", tool.Name, schemes)
		}
		if tool.Meta["openai/profile"] == true {
			profileTool = tool
		}
	}
	if profileTool == nil || !profileTool.Annotations.ReadOnlyHint {
		t.Fatal("no authenticated read-only profile tool")
	}
	raw, _ := json.Marshal(profileTool.OutputSchema)
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	id := properties["id"].(map[string]any)
	if len(properties) != 4 || id["minLength"] != float64(1) || id["pattern"] != `\S` || schema["additionalProperties"] != false {
		t.Fatalf("profile schema does not enforce the host contract: %s", raw)
	}
	profile := mustCall(t, session, "get_profile", nil)
	if profile["id"] == "" || profile["email"] != "owner@example.com" {
		t.Fatalf("profile did not resolve the credential: %v", profile)
	}
	actor, err := e.keys.Authenticate(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := e.keys.Describe(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	reconnect := e.session(t, e.oauth(t, user))
	reconnected := mustCall(t, reconnect, "get_profile", nil)
	if !reflect.DeepEqual(profile, reconnected) {
		t.Fatalf("reconnection changed profile identity: %v %v", profile, reconnected)
	}
	otherKey, _, err := e.keys.CreateKey(ctx, user.ID, "another connection", "")
	if err != nil {
		t.Fatal(err)
	}
	another := e.session(t, otherKey)
	if got := mustCall(t, another, "get_profile", nil); got["id"] != profile["id"] {
		t.Fatalf("a new token changed profile identity: %v", got)
	}
	for uri, modes := range map[string][]string{"ui://jaz-crm/app": {"inline", "fullscreen"}, "ui://jaz-crm/o/people?limit=20": {"inline", "fullscreen"}} {
		resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			t.Fatal(err)
		}
		content := resource.Contents[0]
		ui := content.Meta["ui"].(map[string]any)
		openai := content.Meta["openai/ui"].(map[string]any)
		got, _ := json.Marshal(openai["availableDisplayModes"])
		want, _ := json.Marshal(modes)
		if content.MIMEType != "text/html;profile=mcp-app" || ui["domain"] != "http://crm.test" || string(got) != string(want) || ui["csp"] == nil {
			t.Fatalf("%s has incomplete UI metadata: %v", uri, content.Meta)
		}
	}
}
