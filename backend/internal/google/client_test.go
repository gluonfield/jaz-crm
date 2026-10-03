package google

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func fake(t *testing.T, routes map[string]http.HandlerFunc) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := routes[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusTeapot)
			return
		}
		route(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.Client(), Endpoints{Gmail: srv.URL, Calendar: srv.URL, Meet: srv.URL, UserInfo: srv.URL})
}

func respond(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		io.WriteString(w, body)
	}
}

func wantQuery(t *testing.T, u *url.URL, want url.Values) {
	t.Helper()
	if got := u.Query(); !reflect.DeepEqual(got, want) {
		t.Errorf("%s query = %v, want %v", u.Path, got, want)
	}
}

// quickBackoff retries at once for the rest of the test.
func quickBackoff(t *testing.T) {
	saved := backoff
	backoff = []time.Duration{0, 0}
	t.Cleanup(func() { backoff = saved })
}

// A read turned away by a rate limit is retried until it succeeds.
func TestRetriesRateLimitedReads(t *testing.T) {
	quickBackoff(t)
	calls := 0
	c := fake(t, map[string]http.HandlerFunc{"/gmail/v1/users/me/profile": func(w http.ResponseWriter, r *http.Request) {
		if calls++; calls == 1 {
			respond(http.StatusForbidden, `{"error":{"errors":[{"reason":"rateLimitExceeded"}]}}`)(w, r)
			return
		}
		io.WriteString(w, `{"emailAddress":"a@x.com","historyId":"7"}`)
	}})
	if p, err := c.Profile(t.Context()); err != nil || p.HistoryID != "7" || calls != 2 {
		t.Fatalf("profile %+v, %v after %d calls", p, err, calls)
	}
}

func TestErrors(t *testing.T) {
	quickBackoff(t)
	cases := []struct {
		status    int
		body      string
		sentinel  error
		retryable bool
	}{
		{401, `{"error":{"code":401,"status":"UNAUTHENTICATED"}}`, ErrRevoked, false},
		{404, `{"error":{"code":404,"message":"Requested entity was not found."}}`, ErrNotFound, false},
		{429, `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED"}}`, nil, true},
		{403, `{"error":{"errors":[{"reason":"rateLimitExceeded"}],"details":[{"reason":"RATE_LIMIT_EXCEEDED"}]}}`, nil, true},
		{403, `{"error":{"errors":[{"reason":"insufficientPermissions"}]}}`, nil, false},
		{400, `{"error":{"code":400,"message":"Invalid query"}}`, nil, false},
		{503, `<html>unavailable</html>`, nil, true},
	}
	for _, tc := range cases {
		c := fake(t, map[string]http.HandlerFunc{"/gmail/v1/users/me/profile": respond(tc.status, tc.body)})
		_, err := c.Profile(t.Context())
		if tc.sentinel != nil {
			if !errors.Is(err, tc.sentinel) {
				t.Errorf("%d: err = %v, want %v", tc.status, err, tc.sentinel)
			}
			continue
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Retryable() != tc.retryable {
			t.Errorf("%d %s: err = %v, want APIError retryable=%v", tc.status, tc.body, err, tc.retryable)
		}
	}
}

func TestAuthURL(t *testing.T) {
	config := OAuthConfig{ClientID: "id", ClientSecret: "secret", RedirectURL: "https://crm.example/oauth/google"}
	u, err := url.Parse(config.AuthURL("state1", "verifier1"))
	if err != nil {
		t.Fatal(err)
	}
	wantQuery(t, u, url.Values{
		"client_id":              {"id"},
		"redirect_uri":           {"https://crm.example/oauth/google"},
		"response_type":          {"code"},
		"state":                  {"state1"},
		"scope":                  {strings.Join(Scopes, " ")},
		"access_type":            {"offline"},
		"prompt":                 {"consent"},
		"include_granted_scopes": {"true"},
		"code_challenge":         {oauth2.S256ChallengeFromVerifier("verifier1")},
		"code_challenge_method":  {"S256"},
	})
}

type redirect string

func (host redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = string(host)
	return http.DefaultTransport.RoundTrip(req)
}

func TestRevokedRefreshToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" || r.FormValue("refresh_token") != "rt" || r.FormValue("client_id") != "id" {
			t.Errorf("unexpected request %s %v", r.URL, r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)
	}))
	defer srv.Close()
	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, &http.Client{Transport: redirect(srv.Listener.Addr().String())})
	c := NewClient(OAuthConfig{ClientID: "id", ClientSecret: "secret"}.Client(ctx, "rt"), Production)
	if _, err := c.Profile(t.Context()); !errors.Is(err, ErrRevoked) {
		t.Fatalf("err = %v, want ErrRevoked", err)
	}
}
