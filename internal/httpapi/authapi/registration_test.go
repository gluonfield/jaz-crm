package authapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/internal/workspaces"
	"github.com/gluonfield/jaz-tasks/auth"
)

func TestClientRegistrationLimits(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})
	cases := []struct {
		name     string
		metadata map[string]any
		status   int
	}{
		{"valid", map[string]any{"client_name": "Audit client", "redirect_uris": []string{"https://client.test/callback"}}, http.StatusCreated},
		{"name", map[string]any{"client_name": strings.Repeat("x", 257), "redirect_uris": []string{"https://client.test/callback"}}, http.StatusBadRequest},
		{"redirect count", map[string]any{"redirect_uris": strings.Fields(strings.Repeat("https://client.test/callback ", 9))}, http.StatusBadRequest},
		{"redirect length", map[string]any{"redirect_uris": []string{"https://client.test/" + strings.Repeat("x", 2048)}}, http.StatusBadRequest},
		{"body", map[string]any{"client_name": "small", "redirect_uris": []string{"https://client.test/callback"}, "extra": strings.Repeat("x", 64<<10)}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.metadata)
			if err != nil {
				t.Fatal(err)
			}
			res, err := http.Post(s.url+"/oauth/register", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			_, _ = io.Copy(io.Discard, res.Body)
			if res.StatusCode != tc.status {
				t.Fatalf("registration status %d, want %d", res.StatusCode, tc.status)
			}
		})
	}
}

func TestClientRegistrationBurstIsLimited(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})
	for i := range 11 {
		res, err := http.Post(s.url+"/oauth/register", "application/json", strings.NewReader(`{"client_name":"client", "redirect_uris":["https://client.test/callback"]}`))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
		want := http.StatusCreated
		if i == 10 {
			want = http.StatusTooManyRequests
			if res.Header.Get("Retry-After") == "" {
				t.Fatal("registration rate limit has no retry guidance")
			}
		}
		if res.StatusCode != want {
			t.Fatalf("registration %d returned %d, want %d", i, res.StatusCode, want)
		}
	}
}
