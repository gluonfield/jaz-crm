package authapi

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// Server-rendered pages for flows that happen outside the web app. Their
// palette follows the web app's default tokens; surfaces separate by fill.
var pages = template.Must(template.New("layout").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · Jaz CRM</title>
<style>
:root{--bg:#f5f5f6;--card:#fff;--ink:#1c1d1f;--ink-2:#5f6168;--ink-3:#8f9199;--primary:#5e6ad2;color-scheme:light}
@media (prefers-color-scheme:dark){:root{--bg:#0e0e10;--card:#1a1b1d;--ink:#e6e7ea;--ink-2:#a0a3aa;--ink-3:#6e717a;--primary:#7480ea;color-scheme:dark}}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;background:var(--bg);color:var(--ink);font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",system-ui,sans-serif;-webkit-font-smoothing:antialiased}
main{width:min(360px,calc(100vw - 32px));text-align:center}
.icon{width:44px;height:44px;margin:0 auto 20px;border-radius:12px;background:var(--card);display:grid;place-items:center;box-shadow:0 1px 2px rgb(0 0 0/.08)}
.icon svg{width:24px;height:24px}
h1{margin:0 0 6px;font-size:19px;font-weight:600;letter-spacing:-.01em}p{margin:0;color:var(--ink-2)}
.account{display:flex;align-items:center;gap:10px;margin-top:24px;padding:10px 12px;border-radius:10px;background:var(--card);text-align:left}
.account img,.account span{width:28px;height:28px;border-radius:50%;flex:none}.account span{display:grid;place-items:center;background:var(--primary);color:#fff;font-size:12px;font-weight:600}
.account b{display:block;font-weight:500;font-size:13px}.account small{color:var(--ink-3);font-size:12px}
.actions{display:grid;gap:6px;margin-top:24px}
button,a.button{height:40px;border:0;border-radius:9px;font:inherit;font-weight:500;cursor:pointer;display:grid;place-items:center;text-decoration:none;color:var(--ink-2);background:transparent}
button:hover,a.button:hover{color:var(--ink)}
.primary{background:var(--primary);color:oklch(from var(--primary) clamp(.2,(.58 - l) * 1000,1) 0 h)}.primary:hover{filter:brightness(1.08);color:oklch(from var(--primary) clamp(.2,(.58 - l) * 1000,1) 0 h)}
.muted{margin-top:14px;font-size:12px;color:var(--ink-3)}
</style></head><body><main><div class="icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></svg></div>{{template "body" .}}</main></body></html>`))

var messagePage = template.Must(template.Must(pages.Clone()).Parse(`{{define "body"}}<h1>{{.Title}}</h1><p>{{.Message}}</p>
{{if .Link}}<div class="actions"><a class="button" href="{{.Link}}">Back to Jaz CRM</a></div>{{end}}{{end}}`))

var consentPage = template.Must(template.Must(pages.Clone()).Parse(`{{define "body"}}<h1>Connect {{.Client}}</h1>
<p>{{.Client}} will be able to read and update records in {{.Workspace}}.</p>
<div class="account">{{if .Avatar}}<img src="{{.Avatar}}" alt="">{{else}}<span>{{.Initial}}</span>{{end}}<div><b>{{.Name}}</b><small>{{.User}}</small></div></div>
<form method="post" action="/oauth/authorize">
{{range $k, $v := .Params}}<input type="hidden" name="{{$k}}" value="{{$v}}">{{end}}
<div class="actions"><button class="primary" name="decision" value="allow">Allow</button><button name="decision" value="deny">Cancel</button></div>
</form><p class="muted">Returns to {{.Redirect}}</p>{{end}}`))

func page(w http.ResponseWriter, status int, title, message, link string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = messagePage.Execute(w, map[string]string{"Title": title, "Message": message, "Link": link})
}

func consent(w http.ResponseWriter, svc *auth.Service, r *http.Request, client storage.OAuthClient, req auth.AuthorizeRequest, actor auth.Actor, token string) {
	redirect, _ := url.Parse(req.RedirectURI)
	user, workspace, err := svc.Describe(r.Context(), actor)
	if err != nil {
		page(w, http.StatusInternalServerError, "Something went wrong", "Please try again.", "/")
		return
	}
	params := map[string]string{
		"response_type":         req.ResponseType,
		"client_id":             req.ClientID,
		"redirect_uri":          req.RedirectURI,
		"code_challenge":        req.CodeChallenge,
		"code_challenge_method": req.CodeChallengeMethod,
		"state":                 req.State,
		"scope":                 req.Scope,
		"resource":              req.Resource,
		"csrf":                  token,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	var avatar string
	if user.AvatarURL != nil {
		avatar = *user.AvatarURL
	}
	_ = consentPage.Execute(w, map[string]any{
		"Title":     "Connect " + client.Name,
		"Client":    client.Name,
		"Name":      user.Name,
		"Initial":   strings.ToUpper(user.Name[:1]),
		"Avatar":    avatar,
		"User":      user.Email,
		"Workspace": workspace.Name,
		"Redirect":  redirect.Host,
		"Params":    params,
	})
}

// csrf binds the consent form to the browser session that rendered it.
func csrf(session *http.Cookie) string {
	if session == nil {
		return ""
	}
	sum := sha256.Sum256([]byte("consent:" + session.Value))
	return hex.EncodeToString(sum[:])
}
