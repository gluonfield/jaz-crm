package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gluonfield/jaz-crm/errs"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

const quotaCooldown = time.Hour

func quotaExhausted(code, kind string) bool {
	switch code {
	case "insufficient_quota", "credit_balance_exhausted", "organization_spend_limit_exceeded", "project_spend_limit_exceeded", "organization_usage_limit_exceeded":
		return true
	}
	return kind == "insufficient_quota"
}

func quotaRetryPolicy(r *http.Request, next option.MiddlewareNext) (*http.Response, error) {
	res, err := next(r)
	if err != nil || res.StatusCode < http.StatusBadRequest {
		return res, err
	}
	data, err := io.ReadAll(res.Body)
	res.Body.Close()
	res.Body = io.NopCloser(bytes.NewReader(data))
	if err != nil {
		return res, err
	}
	var body struct{ Error struct{ Code, Type string } }
	if json.Unmarshal(data, &body) == nil && quotaExhausted(body.Error.Code, body.Error.Type) {
		res.Header.Set("x-should-retry", "false")
	}
	return res, nil
}

func (c *Client) response(ctx context.Context, request responses.ResponseNewParams, options ...option.RequestOption) (*responses.Response, error) {
	c.mu.Lock()
	paused := c.paused
	probe := paused != nil && !c.now().Before(paused.RetryAt)
	if probe {
		c.paused = &errs.Deferred{Reason: paused.Reason, RetryAt: c.now().Add(quotaCooldown)}
	}
	c.mu.Unlock()
	if paused != nil {
		if !probe {
			return nil, paused
		}
		_, err := c.api.Responses.New(ctx, responses.ResponseNewParams{
			Model:           c.model,
			Input:           responses.ResponseNewParamsInputUnion{OfString: openai.String("Reply OK.")},
			Reasoning:       shared.ReasoningParam{Effort: c.effort},
			MaxOutputTokens: openai.Int(16),
			Store:           openai.Bool(false),
		}, option.WithMaxRetries(0))
		c.mu.Lock()
		if err == nil {
			c.paused = nil
		}
		paused = c.paused
		c.mu.Unlock()
		if err != nil {
			return nil, paused
		}
		c.logger.Info("model requests resumed")
	}
	res, err := c.api.Responses.New(ctx, request, options...)
	var failure *openai.Error
	if errors.As(err, &failure) && quotaExhausted(failure.Code, failure.Type) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.paused == nil {
			c.paused = &errs.Deferred{Reason: "AI work is paused because the model provider's billing or usage limit was reached. It will check again within an hour.", RetryAt: c.now().Add(quotaCooldown)}
			c.logger.Error("model requests paused", "retry_at", c.paused.RetryAt, "error", err)
		}
		return nil, c.paused
	}
	return res, err
}
