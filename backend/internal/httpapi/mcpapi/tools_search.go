package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type searchQuery struct {
	Query string `json:"query" jsonschema:"text matching a record's values or a page's content"`
}

type searchResult struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	URL   string   `json:"url"`
	Text  string   `json:"text"`
	Meta  mcp.Meta `json:"_meta"`
}

type searchResults struct {
	Results []searchResult `json:"results"`
}

const searchPerObject = 3

// registerSearch follows OpenAI's MCP search convention, which hosts call
// outside a conversation, such as Jaz's command palette. Each result's
// preview target, from OpenAI's MCP extensions, opens it in this server's app.
func registerSearch(r *registry, crm *records.Service) {
	add(r, &mcp.Tool{Name: "search", Title: "Search", Annotations: readOnly,
		Description: "Find people, companies, deals, pages and other records matching a query, the newest few of each object, as names with links. Use search_records to filter one object and read values."},
		func(ctx context.Context, actor auth.Actor, in searchQuery) (searchResults, error) {
			objects, err := crm.Objects(ctx, actor, false)
			if err != nil {
				return searchResults{}, err
			}
			out := searchResults{Results: []searchResult{}}
			var ids []string
			for _, object := range objects {
				matches, _, err := crm.Search(ctx, actor, records.Search{Object: object.Slug, Query: in.Query, Limit: searchPerObject})
				if err != nil {
					return searchResults{}, err
				}
				for _, record := range matches {
					ids = append(ids, record.ID)
					target := map[string]any{"type": "mcp_app_tool", "name": "show_crm", "arguments": showInput{Path: "/r/" + record.ID}}
					out.Results = append(out.Results, searchResult{ID: record.ID, URL: r.publicURL + "/r/" + record.ID, Text: object.Name, Meta: mcp.Meta{"openai/preview": map[string]any{"target": target}}})
				}
			}
			labels, err := crm.Labels(ctx, actor.WorkspaceID, ids)
			for i := range out.Results {
				out.Results[i].Title = labels[out.Results[i].ID].Name
			}
			return out, err
		})
}
