package google

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"

	"golang.org/x/oauth2"
)

var (
	// ErrRevoked means the refresh token no longer works and the account must reconnect.
	ErrRevoked = errors.New("google: authorization revoked")
	// ErrExpiredCursor means a Gmail history id or Calendar sync token is too old; resync from scratch.
	ErrExpiredCursor = errors.New("google: sync cursor expired")
	ErrNotFound      = errors.New("google: not found")
)

var statusErrors = map[int]error{
	http.StatusUnauthorized: ErrRevoked,
	http.StatusNotFound:     ErrNotFound,
	http.StatusGone:         ErrExpiredCursor,
}

type APIError struct {
	Status          int
	Reason, Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("google: %d %s: %s", e.Status, e.Reason, e.Message)
}

func (e *APIError) Retryable() bool {
	transient := slices.Contains([]string{"rateLimitExceeded", "userRateLimitExceeded", "backendError"}, e.Reason)
	return transient || e.Status == http.StatusTooManyRequests || e.Status >= 500
}

// Endpoints are the API base URLs; tests point all three at one httptest server.
type Endpoints struct {
	Gmail, Calendar, Meet string
}

var Production = Endpoints{
	Gmail:    "https://gmail.googleapis.com",
	Calendar: "https://www.googleapis.com",
	Meet:     "https://meet.googleapis.com",
}

type Client struct {
	http      *http.Client
	endpoints Endpoints
}

func NewClient(httpClient *http.Client, endpoints Endpoints) *Client {
	return &Client{http: httpClient, endpoints: endpoints}
}

// get drops empty query values, which Google treats as unset anyway.
func (c *Client) get(ctx context.Context, endpoint string, query url.Values, out any) error {
	maps.DeleteFunc(query, func(_ string, v []string) bool { return len(v) == 1 && v[0] == "" })
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return c.do(ctx, http.MethodGet, endpoint, nil, out)
}

func (c *Client) post(ctx context.Context, endpoint string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, endpoint, bytes.NewReader(raw), out)
}

func (c *Client) do(ctx context.Context, method, endpoint string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	var retrieve *oauth2.RetrieveError
	if errors.As(err, &retrieve) && retrieve.ErrorCode == "invalid_grant" {
		return fmt.Errorf("%w: %w", ErrRevoked, err)
	}
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return responseError(res)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

type errorReason struct {
	Reason string
}

func responseError(res *http.Response) error {
	var body struct {
		Error struct {
			Message, Status string
			Errors, Details []errorReason
		}
	}
	json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&body)
	e := &APIError{Status: res.StatusCode, Message: body.Error.Message}
	for _, r := range slices.Concat(body.Error.Errors, body.Error.Details, []errorReason{{body.Error.Status}}) {
		if r.Reason != "" {
			e.Reason = r.Reason
			break
		}
	}
	if sentinel, ok := statusErrors[res.StatusCode]; ok {
		return fmt.Errorf("%w: %v", sentinel, e)
	}
	return e
}
