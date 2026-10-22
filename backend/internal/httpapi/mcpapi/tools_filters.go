package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerFilters(r *registry, crm *records.Service) {
	add(r, &mcp.Tool{Name: "list_saved_filters", Title: "List saved filters", Annotations: readOnly,
		Description: "List views for an object. Follow Ups includes fixed Everyone and Assigned to me views plus your private filters; other objects share named filters across the workspace. Apply one by passing its query and filters to search_records."},
		func(ctx context.Context, actor auth.Actor, in filterObjectInput) (filtersOutput, error) {
			filters, err := crm.SavedFilters(ctx, actor, in.Object)
			return filtersOutput{Filters: filters}, err
		})
	add(r, &mcp.Tool{Name: "save_filter", Title: "Save filter",
		Description: "Save a named text search and attribute conditions. Follow Ups filters are private to the authenticated user; filters for other objects are shared by the workspace. Everyone and Assigned to me are fixed Follow Ups views. Omit id to create; supply its id to update."},
		func(ctx context.Context, actor auth.Actor, in saveFilterInput) (storage.SavedFilter, error) {
			return crm.SaveFilter(ctx, actor, in.Object, storage.SavedFilter{ID: in.ID, Name: in.Name, Query: in.Query, Filters: in.Filters})
		})
	add(r, &mcp.Tool{Name: "delete_saved_filter", Title: "Delete saved filter",
		Description: "Delete an accessible named saved filter; Follow Ups filters belong to their creator and built-in views cannot be deleted; its records remain."},
		func(ctx context.Context, actor auth.Actor, in filterIDInput) (empty, error) {
			return empty{}, crm.DeleteFilter(ctx, actor, in.ID)
		})
}

type filterObjectInput struct {
	Object string `json:"object"`
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
