package mcpapi

import (
	"context"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerDates(r *registry, crm *records.Service) {
	add(r, &mcp.Tool{Name: "save_action_date", Title: "Save action date", Meta: mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "Explicitly override a follow-up's action date, or clear it with an empty value. The manual override is preserved during automatic drafting, including a cleared date. Accepts YYYY-MM-DD or RFC3339 with an offset."},
		func(ctx context.Context, actor auth.Actor, in actionDateInput) (recordView, error) {
			if strings.TrimSpace(in.RecordID) == "" {
				return recordView{}, errs.Invalidf("record_id is required")
			}
			w := records.Write{Object: records.FollowUps, RecordID: in.RecordID}
			if strings.TrimSpace(in.Value) == "" {
				w.Remove = map[string][]string{"action_date": nil}
			} else {
				w.Set = map[string][]string{"action_date": {in.Value}}
			}
			record, _, err := crm.Upsert(ctx, actor, records.SourceUser, w)
			return recordOf(record, nil), err
		})
}

type actionDateInput struct {
	RecordID string `json:"record_id"`
	Value    string `json:"value"`
}
