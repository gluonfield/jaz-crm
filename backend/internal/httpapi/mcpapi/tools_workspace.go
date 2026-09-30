package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerWorkspace(r *registry, members *workspaces.Service) {
	add(r, &mcp.Tool{Name: "get_workspace", Title: "Get workspace", Annotations: readOnly,
		Description: "Describe this workspace: its name, description, members and pending invites."},
		func(ctx context.Context, actor auth.Actor, _ empty) (workspaceView, error) {
			workspace, err := members.Workspace(ctx, actor)
			if err != nil {
				return workspaceView{}, err
			}
			users, invites, err := members.Members(ctx, actor)
			out := workspaceView{Name: workspace.Name, Description: workspace.Description, Members: []memberView{}, Invited: []string{}}
			for _, u := range users {
				out.Members = append(out.Members, memberView{Name: u.Name, Email: u.Email, Admin: u.Admin, IsMe: u.ID == actor.UserID})
			}
			for _, i := range invites {
				out.Invited = append(out.Invited, i.Email)
			}
			return out, err
		})
	add(r, &mcp.Tool{Name: "invite_member", Title: "Invite member",
		Description: "Invite someone to this workspace by email; they join when they next sign in. Admins only."},
		func(ctx context.Context, actor auth.Actor, in inviteInput) (inviteOutput, error) {
			invite, err := members.Invite(ctx, actor, in.Email)
			return inviteOutput{Invited: invite.Email}, err
		})
	add(r, &mcp.Tool{Name: "update_workspace", Title: "Update workspace",
		Description: "Change this workspace's name or description. The description tells triage which contacts belong in the CRM. Admins only."},
		func(ctx context.Context, actor auth.Actor, in updateWorkspaceInput) (workspaceView, error) {
			workspace, err := members.Update(ctx, actor, in.Name, in.Description)
			return workspaceView{Name: workspace.Name, Description: workspace.Description}, err
		})
}

type memberView struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Admin bool   `json:"admin,omitempty"`
	IsMe  bool   `json:"is_me,omitempty"`
}

type workspaceView struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Members     []memberView `json:"members,omitempty"`
	Invited     []string     `json:"invited,omitempty"`
}

type inviteInput struct {
	Email string `json:"email"`
}

type inviteOutput struct {
	Invited string `json:"invited"`
}

type updateWorkspaceInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty" jsonschema:"who this workspace's CRM is for, such as: manufacturing customers, suppliers and partners"`
}
