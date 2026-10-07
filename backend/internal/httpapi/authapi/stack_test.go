package authapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/logosapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/pageimagesapi"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-tasks/auth"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/authapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/connectapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/webhooks"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/server"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type stack struct {
	url    string
	apiKey string
	keys   *auth.Service
	owner  string
}

// start runs the whole HTTP stack on a loopback URL that doubles as PUBLIC_URL.
func start(t *testing.T, oidc signin.OIDCConfig, members workspaces.Config) stack {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	base := "http://" + srv.Listener.Addr().String()
	store := postgrestest.New(t)
	keys := auth.NewService(store, auth.Config{PublicURL: base})
	oidc.RedirectURL = base + "/auth/callback"
	people := workspaces.NewService(store, members)
	owner, err := people.Provision(context.Background(), "owner@jaz.test")
	if err != nil {
		t.Fatal(err)
	}
	apiKey, _, err := keys.CreateKey(context.Background(), owner.ID, "test", "")
	if err != nil {
		t.Fatal(err)
	}
	authn, err := authapi.NewHandler(keys, people, signin.Config{PublicURL: base, OIDC: oidc}, log.New(io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	logger := log.New(io.Discard)
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	conns, err := connections.NewService(store, connections.Config{}, idle{})
	if err != nil {
		t.Fatal(err)
	}
	lg := logos.NewService(store, logos.Fetcher{})
	agents := mcpapi.NewHandler(mcpapi.Services{Records: crm, Workspaces: people, Interactions: convs, Connections: conns, Logos: lg}, keys, logger)
	srv.Config.Handler = server.New(authn, agents, connectapi.NewHandler(conns, keys, logger), webhooks.NewHandler(conns, idle{}, convs, keys, webhooks.Config{}, logger), logosapi.NewHandler(lg, logger), pageimagesapi.NewHandler(crm, logger), "")
	srv.Start()
	t.Cleanup(srv.Close)
	return stack{url: base, apiKey: apiKey, keys: keys, owner: owner.ID}
}

// browser keeps cookies and hands redirects back instead of following them.
func browser() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// ownerSession signs the browser in as the provisioned workspace's owner.
func (s stack) ownerSession(t *testing.T, b *http.Client) {
	t.Helper()
	token, _, err := s.keys.CreateSession(context.Background(), s.owner)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse(s.url)
	b.Jar.SetCookies(base, []*http.Cookie{{Name: "jc_session", Value: token, Path: "/"}})
}

var hiddenInput = regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`)

// consent opens an authorization URL in a signed-in browser, approves it and
// returns the redirect the client receives.
func consent(t *testing.T, b *http.Client, authorizeURL string) *url.URL {
	t.Helper()
	res, err := b.Get(authorizeURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `value="allow"`) {
		t.Fatalf("consent page: %d %s", res.StatusCode, body)
	}
	form := url.Values{"decision": {"allow"}}
	for _, m := range hiddenInput.FindAllStringSubmatch(string(body), -1) {
		form.Set(m[1], htmlUnescape(m[2]))
	}
	u, _ := url.Parse(authorizeURL)
	res, err = b.PostForm(u.Scheme+"://"+u.Host+"/oauth/authorize", form)
	if err != nil || res.StatusCode != http.StatusSeeOther {
		t.Fatalf("approve: %v %v", res.StatusCode, err)
	}
	res.Body.Close()
	location, _ := url.Parse(res.Header.Get("Location"))
	return location
}

func htmlUnescape(s string) string {
	return strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&#34;", `"`, "&#39;", "'", "&#43;", "+").Replace(s)
}

// agent connects to /mcp with an API key the browser's person creates.
func (s stack) agent(t *testing.T, b *http.Client) *mcp.ClientSession {
	t.Helper()
	_, body := session(t, s, b, http.MethodPost, "/auth/api-keys", `{"label":"agent"}`)
	var created struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.Key == "" {
		t.Fatalf("create key: %s", body)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "1"}, nil)
	conn, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   s.url + "/mcp",
		HTTPClient: &http.Client{Transport: bearer(created.Key)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

type bearer string

func (key bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+string(key))
	return http.DefaultTransport.RoundTrip(req)
}

// callText calls a tool and returns its text result, reporting tool errors too.
func callText(t *testing.T, conn *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := conn.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res.Content[0].(*mcp.TextContent).Text
}

// idle stands in for the Temporal worker, which these tests never reach.
type idle struct{}

func (idle) Start(context.Context, string) error          { return nil }
func (idle) Stop(context.Context, string) error           { return nil }
func (idle) Step(context.Context, string) (string, error) { return "", nil }
