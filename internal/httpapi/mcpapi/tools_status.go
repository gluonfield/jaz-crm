package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/internal/auth"
	"github.com/gluonfield/jaz-crm/internal/status"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerStatus(r *registry, service *status.Service) {
	add(r, &mcp.Tool{Name: "get_operation_errors", Title: "Background issues", Annotations: readOnly,
		Description: "Current reported AI background issues and sync failures for your connections in this workspace, cleared after observed recovery."},
		func(ctx context.Context, actor auth.Actor, _ empty) (operationErrors, error) {
			errors, err := service.Errors(ctx, actor.WorkspaceID, actor.MemberID)
			return operationErrors{Errors: errors}, err
		})
}

type operationErrors struct {
	Errors []status.Error `json:"errors"`
}
