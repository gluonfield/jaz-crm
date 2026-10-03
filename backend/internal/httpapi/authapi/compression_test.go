package authapi_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	signin "github.com/gluonfield/jaz-tasks/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOriginCompression(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "compression-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   s.url + "/mcp",
		HTTPClient: &http.Client{Transport: bearer(s.apiKey)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	uri := "ui://jaz-crm/app"
	expected, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil || expected == nil || len(expected.Contents) != 1 || len(expected.Contents[0].Text) < 100_000 {
		t.Fatalf("native MCP client could not read the real app: %v", err)
	}
	transport := &http.Transport{DisableCompression: true}
	defer transport.CloseIdleConnections()
	rawClient := &http.Client{Transport: transport}
	var identityBytes int
	for _, encoding := range []string{"identity", "gzip"} {
		input, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 200, "method": "resources/read",
			"params": map[string]string{"uri": uri},
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.url+"/mcp", bytes.NewReader(input))
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
		req.Header.Set("Mcp-Session-Id", session.ID())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Accept-Encoding", encoding)
		res, err := rawClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("resource HTTP response: %d %v", res.StatusCode, err)
		}
		data := raw
		if encoding == "identity" {
			identityBytes = len(raw)
			if res.Header.Get("Content-Encoding") != "" {
				t.Fatal("identity response was compressed")
			}
		} else {
			if res.Header.Get("Content-Encoding") != "gzip" || len(raw) >= identityBytes/2 {
				t.Fatalf("origin did not compress app: %d -> %d bytes, %v", identityBytes, len(raw), res.Header)
			}
			reader, err := gzip.NewReader(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			data, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("app response: %d -> %d bytes (%.1f%% smaller)", identityBytes, len(raw), 100*(1-float64(len(raw))/float64(identityBytes)))
		}
		payload := data
		if strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
			for line := range strings.SplitSeq(string(data), "\n") {
				if after, ok := strings.CutPrefix(line, "data: "); ok {
					payload = []byte(after)
					break
				}
			}
		}
		var message struct {
			Result mcp.ReadResourceResult `json:"result"`
		}
		if err := json.Unmarshal(payload, &message); err != nil || len(message.Result.Contents) != 1 || message.Result.Contents[0].Text != expected.Contents[0].Text {
			t.Fatalf("%s altered the app resource: %v", encoding, err)
		}
	}
}
