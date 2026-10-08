package mcpapi

import (
	"context"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerTrash(r *registry, crm *records.Service) {
	add(r, &mcp.Tool{Name: "list_trash", Title: "List Trash", Annotations: readOnly,
		Description: "List records and pages in the current workspace's Trash, newest deletion first. Restore them with restore_record."},
		func(ctx context.Context, actor auth.Actor, _ empty) (trashOutput, error) {
			rows, err := crm.Trash(ctx, actor)
			out := trashOutput{Records: []trashView{}}
			for _, row := range rows {
				out.Records = append(out.Records, trashView{ID: row.ID, Object: row.Object, Name: row.Name, Icon: r.icon(row.Icon), DeletedAt: row.DeletedAt})
			}
			return out, err
		})
	add(r, &mcp.Tool{Name: "restore_record", Title: "Restore record", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)},
		Description: "Restore a record from Trash with its original ID, values, history and references. Addresses skipped by its deletion return to Kept; newer triage decisions are preserved."},
		func(ctx context.Context, actor auth.Actor, in recordInput) (empty, error) {
			return empty{}, crm.Restore(ctx, actor, in.RecordID)
		})
}

type trashOutput struct {
	Records []trashView `json:"records"`
}

type trashView struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon,omitempty"`
	DeletedAt time.Time `json:"deleted_at"`
}
