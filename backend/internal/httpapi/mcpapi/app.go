package mcpapi

import (
	"context"
	_ "embed"
	"encoding/base64"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// appHTML is the web app built as one file for MCP hosts (bun run build).
//
//go:embed app/mcp-app.html
var appHTML string

const (
	appURI  = "ui://jaz-crm/app"
	appMIME = "text/html;profile=mcp-app"
)

// glyph is two people, line-drawn so hosts can tint it.
const glyph = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="STROKE" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="9" cy="7" r="4"/><path d="M2 21v-2a4 4 0 0 1 4-4h6a4 4 0 0 1 4 4v2M16 3.13a4 4 0 0 1 0 7.75M22 21v-2a4 4 0 0 0-3-3.87"/></svg>`

func icon(stroke string) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(strings.Replace(glyph, "STROKE", stroke, 1)))
}

var icons = []mcp.Icon{
	{Source: icon("#1f2328"), MIMEType: "image/svg+xml", Sizes: []string{"any"}, Theme: mcp.IconThemeLight},
	{Source: icon("#e8e8e8"), MIMEType: "image/svg+xml", Sizes: []string{"any"}, Theme: mcp.IconThemeDark},
}

// registerApp publishes the web app as an MCP App that hosts show in their
// sidebar, opened at a page by show_crm.
func registerApp(r *registry) {
	r.server.AddResource(&mcp.Resource{URI: appURI, Name: "jaz-crm", Title: "Jaz CRM", MIMEType: appMIME},
		func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
				URI: appURI, MIMEType: appMIME, Text: appHTML, Meta: mcp.Meta{"ui": map[string]any{
					"prefersBorder": false,
					// Profile pictures load from Google.
					"csp": map[string]any{"resourceDomains": []string{"https://*.googleusercontent.com"}},
				}},
			}}}, nil
		})
	add(r, &mcp.Tool{Name: "show_crm", Title: "CRM", Annotations: readOnly, Icons: []mcp.Icon{{Source: icon("currentColor"), MIMEType: "image/svg+xml", Sizes: []string{"any"}}},
		Description: "Open the CRM app at a page: /o/people, /r/<record id>, /i/<interaction id>, /triage, /connections or /settings.",
		Meta: mcp.Meta{
			"ui":             map[string]any{"resourceUri": appURI},
			"ui/resourceUri": appURI,
			"openai/ui":      map[string]any{"entrypoints": []map[string]any{{"type": "global"}}},
		}},
		func(_ context.Context, _ auth.Actor, in showInput) (showOutput, error) {
			if !strings.HasPrefix(in.Path, "/") {
				in.Path = "/"
			}
			return showOutput{Path: in.Path}, nil
		})
}

type showInput struct {
	Path string `json:"path,omitempty" jsonschema:"the page to open, such as /r/<record id>"`
}

type showOutput struct {
	Path string `json:"path"`
}
