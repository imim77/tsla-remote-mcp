package tesla

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"tsla-remote-mcp/internal/store"
)

type Client struct {
	baseURL    string
	tokens     store.Repository
	httpClient *http.Client
}

func NewClient(baseURL string, tokens store.Repository) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		tokens:  tokens,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) ListVehicles(ctx context.Context) (json.RawMessage, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.Path != "" || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("TESLA_AUDIENCE must be a Fleet API HTTPS base URL")
	}
	if c.tokens == nil {
		return nil, errors.New("token store is not configured")
	}
	token, err := c.tokens.Load(ctx)
	if errors.Is(err, store.ErrTokenNotFound) {
		return nil, errors.New("sign in to Tesla through /auth/tsla first")
	}
	if err != nil {
		return nil, fmt.Errorf("load Tesla token: %w", err)
	}
	if !token.Valid() {
		return nil, errors.New("Tesla token has expired or is empty; sign in through /auth/tsla again")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/1/vehicles", nil)
	if err != nil {
		return nil, err
	}
	token.SetAuthHeader(request)
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Tesla vehicles: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tesla vehicles request failed: HTTP %d", response.StatusCode)
	}
	const maxResponseBytes = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Tesla response: %w", err)
	}
	if len(body) > maxResponseBytes || !json.Valid(body) {
		return nil, errors.New("Tesla returned an invalid or oversized JSON response")
	}
	return json.RawMessage(body), nil
}
