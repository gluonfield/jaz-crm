package mcpapi

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"html"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// appHTML is the web app built as one file for MCP hosts (bun run build).
//
//go:embed app/mcp-app.html
var appHTML string

//go:embed app/favicon.ico
var faviconICO []byte

const (
	appURI  = "ui://jaz-crm/app"
	appMIME = "text/html;profile=mcp-app"
)

// glyph is two people, line-drawn so hosts can tint it.
const glyph = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="STROKE" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="9" cy="7" r="4"/><path d="M2 21v-2a4 4 0 0 1 4-4h6a4 4 0 0 1 4 4v2M16 3.13a4 4 0 0 1 0 7.75M22 21v-2a4 4 0 0 0-3-3.87"/></svg>`

func icon(stroke string) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(strings.Replace(glyph, "STROKE", stroke, 1)))
}

// favicon is the glyph for browser tabs, which follows the tab's colour scheme.
var favicon = strings.Replace(strings.Replace(glyph, "STROKE", "#1f2328", 1), "><", "><style>@media (prefers-color-scheme: dark){svg{stroke:#e8e8e8}}</style><", 1)

func Favicon(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		w.Header().Set("Content-Type", "image/x-icon")
		_, _ = w.Write(faviconICO)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = io.WriteString(w, favicon)
}

var icons = []mcp.Icon{
	{Source: icon("#1f2328"), MIMEType: "image/svg+xml", Sizes: []string{"any"}, Theme: mcp.IconThemeLight},
	{Source: icon("#e8e8e8"), MIMEType: "image/svg+xml", Sizes: []string{"any"}, Theme: mcp.IconThemeDark},
}

func registerApp(r *registry, publicURL string) {
	readApp := func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		body := strings.Replace(appHTML, `id="root"`, `id="root" data-mcp-url="`+html.EscapeString(publicURL+"/mcp")+`"`, 1)
		if req.Params.URI != appURI {
			uri, err := url.Parse(req.Params.URI)
			if err != nil {
				return nil, err
			}
			body = strings.Replace(body, `id="root"`, `id="root" data-start-path="`+html.EscapeString(uri.RequestURI())+`"`, 1)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: req.Params.URI, MIMEType: appMIME, Text: body, Meta: mcp.Meta{"ui": map[string]any{
				"prefersBorder": false,
				"domain":        publicURL,
				// User-linked page icons can come from any HTTP(S) origin.
				"csp": map[string]any{"connectDomains": []string{}, "resourceDomains": []string{"https://*:*", "http://*:*"}},
			}, "openai/ui": map[string]any{"availableDisplayModes": []string{"inline", "fullscreen"}}},
		}}}, nil
	}
	r.server.AddResource(&mcp.Resource{URI: appURI, Name: "jaz-crm", Title: "Jaz CRM", MIMEType: appMIME}, readApp)
	r.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "ui://jaz-crm/o/{object}{?q,saved,limit,view,group_by_conversation,conversation_id}", Name: "crm-records", Title: "CRM records", MIMEType: appMIME,
		Description: `Render an object's records with URL filters. q is text and saved is a saved view's id. Every other key filters by an attribute, all of which must match: stage=Lead for an exact value, attribute.operator=value for is_not, contains, not_contains, before, on_or_before, after or on_or_after, and attribute.is_empty or attribute.is_not_empty; repeat a key for several values. limit is at most 100; view=table selects a table. For follow_ups, group_by_conversation selects one row per conversation and conversation_id limits actions to that conversation. URL-encode query values.`,
	}, readApp)
	r.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "ui://jaz-crm/r/{record_id}", Name: "crm-record", Title: "CRM record", MIMEType: appMIME,
		Description: "Render a compact person, company, deal or custom record card. Clicking it opens the full CRM record. Requires access to the record's workspace.",
	}, readApp)
	addUnscoped(r, &mcp.Tool{Name: "show_crm", Title: "Customers", Annotations: readOnly, Icons: []mcp.Icon{{Source: icon("currentColor"), MIMEType: "image/svg+xml", Sizes: []string{"any"}}},
		Description: "Open the CRM app at a page: /o/people, /o/companies, /o/deals, /o/<table>, /r/<record id> for a record or page (with ?tab=activity for its changes), /i/<interaction id>, /triage, /connections or /settings. Also accepts ui://jaz-crm/r/<record id> for a record card, or a filtered ui://jaz-crm/o/<object> resource URL. Record lists accept q for text, saved for a saved view's id, attribute=value or attribute.operator=value filters such as ?tags=Lead or ?updated_at.after=2026-10-01, limit up to 100, and view=table. get_record and search_records return the matching resource_uri and open it automatically.",
		Meta: mcp.Meta{
			"ui":             map[string]any{"resourceUri": appURI},
			"ui/resourceUri": appURI,
			"openai/ui":      map[string]any{"entrypoints": []map[string]any{{"type": "global"}}},
		}},
		func(_ context.Context, _ auth.Actor, in showInput) (showOutput, error) {
			if !strings.HasPrefix(in.Path, "/") && !strings.HasPrefix(in.Path, "ui://jaz-crm/o/") && !strings.HasPrefix(in.Path, "ui://jaz-crm/r/") {
				in.Path = "/"
			}
			return showOutput{Path: in.Path}, nil
		})
}

type showInput struct {
	Path string `json:"path,omitempty" jsonschema:"the page or filtered resource URL to open, such as /r/<record id> or ui://jaz-crm/o/deals?stage=Lead"`
}

type showOutput struct {
	Path string `json:"path"`
}

// listKeys are a record list's own URL keys. Every other key is a filter:
// attribute=value, or attribute.operator=value for any other operator.
var listKeys = []string{"q", "sort", "view", "limit", "saved", "filters", "where", "category", "group_by_conversation", "conversation_id"}

func recordSearchURI(in searchInput) string {
	params := url.Values{}
	if in.Sort != "" {
		params.Set("sort", in.Sort)
	}
	if in.Query != "" {
		params.Set("q", searchValue(in.Query))
	}
	conditions := slices.Clone(in.Filters)
	for _, attribute := range slices.Sorted(maps.Keys(in.Where)) {
		conditions = append(conditions, records.Filter{Attribute: attribute, Operator: "is", Value: in.Where[attribute]})
	}
	rest := []records.Filter{}
	for _, f := range conditions {
		if slices.Contains(listKeys, f.Attribute) || strings.Contains(f.Attribute, ".") {
			rest = append(rest, f)
			continue
		}
		key := f.Attribute
		if f.Operator != "is" {
			key += "." + f.Operator
		}
		params.Add(key, searchValue(f.Value))
	}
	// Follow-ups list open ones by default; an empty list shows them all.
	if len(rest) > 0 || in.Object == records.FollowUps && len(conditions) == 0 {
		filters, _ := json.Marshal(rest)
		params.Set("filters", string(filters))
	}
	if in.Object == records.FollowUps {
		params.Set("group_by_conversation", strconv.FormatBool(in.GroupByConversation))
		if !in.GroupByConversation {
			params.Set("view", "table")
		}
	}
	if in.ConversationID != "" {
		params.Set("conversation_id", in.ConversationID)
	}
	if in.Limit > 0 {
		params.Set("limit", strconv.Itoa(min(in.Limit, 100)))
	} else {
		params.Set("limit", "20")
	}
	return "ui://jaz-crm/o/" + url.PathEscape(in.Object) + "?" + strings.ReplaceAll(params.Encode(), "+", "%20")
}

// searchValue writes text the app reads back as the same text; the app reads
// a value as JSON when it can, so text that is valid JSON, such as 42, is
// quoted.
func searchValue(text string) string {
	if json.Valid([]byte(text)) {
		quoted, _ := json.Marshal(text)
		return string(quoted)
	}
	return text
}
