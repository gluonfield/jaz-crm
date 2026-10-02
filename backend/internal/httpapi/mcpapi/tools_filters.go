package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerFilters(r *registry, crm *records.Service) {
	add(r, &mcp.Tool{Name: "get_active_filter", Title: "Get workspace filter", Annotations: readOnly,
		Description: "Get the active text search and conditions for an object's list, shared by all workspace members."},
		func(ctx context.Context, actor auth.Actor, in filterObjectInput) (storage.ActiveFilter, error) {
			return crm.ActiveFilter(ctx, actor, in.Object)
		})
	add(r, &mcp.Tool{Name: "set_active_filter", Title: "Set workspace filter",
		Description: "Replace the active text search and conditions for an object's list for every workspace member. Empty filters and query show all records. saved_id optionally identifies the named preset being edited."},
		func(ctx context.Context, actor auth.Actor, in activeFilterInput) (storage.ActiveFilter, error) {
			return crm.SetActiveFilter(ctx, actor, in.Object, storage.ActiveFilter{Query: in.Query, Filters: in.Filters, SavedID: in.SavedID})
		})
	add(r, &mcp.Tool{Name: "list_saved_filters", Title: "List saved filters", Annotations: readOnly,
		Description: "List named filters for an object in this workspace. Apply one by passing its query and filters to search_records."},
		func(ctx context.Context, actor auth.Actor, in filterObjectInput) (filtersOutput, error) {
			filters, err := crm.SavedFilters(ctx, actor, in.Object)
			return filtersOutput{Filters: filters}, err
		})
	add(r, &mcp.Tool{Name: "save_filter", Title: "Save filter",
		Description: "Save a named text search and attribute conditions for an object, shared by the workspace. Omit id to create; supply its id to update."},
		func(ctx context.Context, actor auth.Actor, in saveFilterInput) (storage.SavedFilter, error) {
			return crm.SaveFilter(ctx, actor, in.Object, storage.SavedFilter{ID: in.ID, Name: in.Name, Query: in.Query, Filters: in.Filters})
		})
	add(r, &mcp.Tool{Name: "delete_saved_filter", Title: "Delete saved filter",
		Description: "Delete a named saved filter from this workspace; its records remain."},
		func(ctx context.Context, actor auth.Actor, in filterIDInput) (empty, error) {
			return empty{}, crm.DeleteFilter(ctx, actor, in.ID)
		})
}

type filterObjectInput struct {
	Object string `json:"object"`
}

type activeFilterInput struct {
	Object  string           `json:"object"`
	Query   string           `json:"query,omitempty"`
	Filters []records.Filter `json:"filters,omitempty"`
	SavedID string           `json:"saved_id,omitempty"`
}

type filterIDInput struct {
	ID string `json:"id"`
}

type saveFilterInput struct {
	Object  string           `json:"object"`
	ID      string           `json:"id,omitempty"`
	Name    string           `json:"name"`
	Query   string           `json:"query,omitempty"`
	Filters []records.Filter `json:"filters,omitempty"`
}

type filtersOutput struct {
	Filters []storage.SavedFilter `json:"filters"`
}
