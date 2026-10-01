package mcpapi

import (
	"context"
	"encoding/json"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type profile struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Nickname string `json:"nickname,omitempty"`
}

func registerProfile(r *registry, keys *auth.Service) {
	add(r, &mcp.Tool{
		Name: "get_profile", Title: "Connected account",
		Description: "Identify the authenticated account and the workspace this call acts in. The profile ID identifies this workspace membership and stays the same across token refresh and reconnection.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Meta:        mcp.Meta{"openai/profile": true},
		OutputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"id":{"type":"string","minLength":1,"pattern":"\\S"},
				"name":{"type":"string"},
				"email":{"type":"string"},
				"nickname":{"type":"string"}
			},
			"required":["id"],
			"additionalProperties":false
		}`),
	}, func(ctx context.Context, actor auth.Actor, _ empty) (profile, error) {
		user, workspace, err := keys.Describe(ctx, actor)
		return profile{ID: user.ID, Name: user.Name, Email: user.Email, Nickname: workspace.Name}, err
	})
}
