package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerWorkspace(r *registry, members *workspaces.Service, keys *auth.Service) {
	add(r, &mcp.Tool{Name: "get_triage_settings", Title: "Triage settings", Annotations: readOnly,
		Description: "Workspace auto-approval rules. All are off by default; explicit address and domain decisions still apply."},
		func(ctx context.Context, actor auth.Actor, _ empty) (storage.TriageSettings, error) {
			return members.TriageSettings(ctx, actor)
		})
	add(r, &mcp.Tool{Name: "update_triage_settings", Title: "Update triage settings",
		Description: "Replace all four workspace auto-approval settings. Email and completed-meeting rules apply only to conversations of at most 10 participants. AI requires a configured classifier and nonempty Who belongs criteria in the workspace description. Admins only; enabling rules also processes existing pending contacts."},
		func(ctx context.Context, actor auth.Actor, in storage.TriageSettings) (empty, error) {
			return empty{}, members.UpdateTriageSettings(ctx, actor, in)
		})
	addUnscoped(r, &mcp.Tool{Name: "list_workspaces", Title: "List workspaces", Annotations: readOnly,
		Description: "The workspaces you belong to. Other tools act in the default one unless their workspace argument names another."},
		func(ctx context.Context, actor auth.Actor, _ empty) (workspacesOutput, error) {
			list, err := members.Memberships(ctx, actor)
			out := workspacesOutput{Workspaces: []workspaceRef{}}
			for _, m := range list {
				out.Workspaces = append(out.Workspaces, workspaceRef{ID: m.WorkspaceID, Name: m.Name, Default: m.WorkspaceID == actor.WorkspaceID})
			}
			return out, err
		})
	addUnscoped(r, &mcp.Tool{Name: "switch_workspace", Title: "Switch workspace", Meta: mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "Make another of your workspaces the default for this connection or web session, as the workspace menu does. Used by the Jaz CRM app."},
		func(ctx context.Context, actor auth.Actor, in workspaceInput) (workspaceRef, error) {
			m, err := members.Member(ctx, actor, in.WorkspaceID)
			if err != nil {
				return workspaceRef{}, err
			}
			return workspaceRef{ID: m.WorkspaceID, Name: m.Name, Default: true}, keys.Switch(ctx, actor, m.UserID)
		})
	addUnscoped(r, &mcp.Tool{Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)}, Name: "create_workspace", Title: "Create workspace",
		Description: "Start a workspace with you as its admin, with people, companies and deals; it syncs nothing until someone connects Google in it. Name it in other tools' workspace argument to work there; the default stays the same."},
		func(ctx context.Context, actor auth.Actor, in createWorkspaceInput) (workspaceRef, error) {
			m, err := members.Create(ctx, actor, in.Name)
			return workspaceRef{ID: m.WorkspaceID, Name: m.Name}, err
		})
	add(r, &mcp.Tool{Name: "get_workspace", Title: "Get workspace", Annotations: readOnly,
		Description: "Describe this workspace: its name, description, members and pending invites."},
		func(ctx context.Context, actor auth.Actor, _ empty) (workspaceView, error) {
			workspace, err := members.Workspace(ctx, actor)
			if err != nil {
				return workspaceView{}, err
			}
			users, invites, err := members.Members(ctx, actor)
			out := workspaceOf(workspace)
			out.Members, out.Invited = []memberView{}, []string{}
			for _, u := range users {
				out.Members = append(out.Members, memberOf(u, actor))
			}
			for _, i := range invites {
				out.Invited = append(out.Invited, i.Email)
			}
			return out, err
		})
	add(r, &mcp.Tool{Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)}, Name: "invite_member", Title: "Invite member",
		Description: "Invite someone to this workspace by email; they join when they next sign in. Admins only."},
		func(ctx context.Context, actor auth.Actor, in inviteInput) (inviteOutput, error) {
			invite, err := members.Invite(ctx, actor, in.Email)
			return inviteOutput{Invited: invite.Email}, err
		})
	add(r, &mcp.Tool{Name: "update_member", Title: "Update member",
		Description: "Replace the other addresses a member sends from, such as a university or personal mailbox, so their mail from them reads as the workspace's own: sent rather than received, never a contact. Their domains stay outside the workspace. Members change their own; admins change anyone's."},
		func(ctx context.Context, actor auth.Actor, in updateMemberInput) (memberView, error) {
			user, err := members.SetAddresses(ctx, actor, in.Email, in.Addresses)
			return memberOf(user, actor), err
		})
	add(r, &mcp.Tool{Name: "update_workspace", Title: "Update workspace",
		Description: "Change this workspace's name, triage description, drafting knowledge pages or web access. Knowledge accepts pages only and includes descendants. Omitted settings are preserved. Admins only."},
		func(ctx context.Context, actor auth.Actor, in updateWorkspaceInput) (workspaceView, error) {
			if in.CompanyPageID != nil {
				if in.CompanyPageIDs != nil {
					return workspaceView{}, errs.Invalidf("use company_page_ids or company_page_id, not both")
				}
				in.CompanyPageIDs = []string{}
				if *in.CompanyPageID != "" {
					in.CompanyPageIDs = []string{*in.CompanyPageID}
				}
			}
			workspace, err := members.Update(ctx, actor, storage.WorkspaceUpdate{Name: in.Name, Description: in.Description, CompanyPageIDs: in.CompanyPageIDs, DraftingWebAccess: in.DraftingWebAccess})
			return workspaceOf(workspace), err
		})
	add(r, &mcp.Tool{Name: "delete_workspace", Title: "Delete workspace", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Permanently delete the workspace this call acts in and all its CRM data, memberships, connections and credentials. Admins only. Confirm with its ID and current name from get_workspace. Every connection to it loses access; other workspaces are preserved."},
		func(ctx context.Context, actor auth.Actor, in deleteWorkspaceInput) (empty, error) {
			return empty{}, members.Delete(ctx, actor, in.WorkspaceID, in.Name)
		})
}

type memberView struct {
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	Addresses []string `json:"addresses,omitempty"`
	Admin     bool     `json:"admin,omitempty"`
	IsMe      bool     `json:"is_me,omitempty"`
	Photo     string   `json:"photo,omitempty"`
}

func memberOf(u storage.User, actor auth.Actor) memberView {
	m := memberView{Name: u.Name, Email: u.Email, Addresses: u.Addresses, Admin: u.Admin, IsMe: u.ID == actor.UserID}
	if u.AvatarURL != nil {
		m.Photo = *u.AvatarURL
	}
	return m
}

type updateMemberInput struct {
	Email     string   `json:"email" jsonschema:"the member's sign-in email, as get_workspace lists it"`
	Addresses []string `json:"addresses" jsonschema:"every other address they send from; an empty list removes them all"`
}

func workspaceOf(w storage.Workspace) workspaceView {
	out := workspaceView{ID: w.ID, Name: w.Name, Description: w.Description, CompanyPageIDs: w.CompanyPageIDs, DraftingWebAccess: w.DraftingWebAccess}
	if len(w.CompanyPageIDs) > 0 {
		out.CompanyPageID = &w.CompanyPageIDs[0]
	}
	return out
}

type workspaceView struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Description       string       `json:"description"`
	CompanyPageID     *string      `json:"company_page_id,omitempty"`
	CompanyPageIDs    []string     `json:"company_page_ids"`
	DraftingWebAccess bool         `json:"drafting_web_access"`
	Members           []memberView `json:"members,omitempty"`
	Invited           []string     `json:"invited,omitempty"`
}

type inviteInput struct {
	Email string `json:"email"`
}

type inviteOutput struct {
	Invited string `json:"invited"`
}

type updateWorkspaceInput struct {
	CompanyPageIDs    []string `json:"company_page_ids,omitempty" jsonschema:"knowledge page IDs; includes their descendants; empty removes all; omission preserves the selection; only pages in this workspace are accepted"`
	DraftingWebAccess *bool    `json:"drafting_web_access,omitempty" jsonschema:"allow the drafting agent to search and read the public web; disabled by default"`
	Name              *string  `json:"name,omitempty"`
	Description       *string  `json:"description,omitempty" jsonschema:"who this workspace's CRM is for, such as: manufacturing customers, suppliers and partners"`
	CompanyPageID     *string  `json:"company_page_id,omitempty" jsonschema:"page ID whose full subtree supplies our company knowledge for drafting; empty clears it; omission preserves it"`
}

type workspaceRef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Default bool   `json:"default,omitempty"`
}

type workspacesOutput struct {
	Workspaces []workspaceRef `json:"workspaces"`
}

type workspaceInput struct {
	WorkspaceID string `json:"workspace_id"`
}

type createWorkspaceInput struct {
	Name string `json:"name"`
}

type deleteWorkspaceInput struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
}
