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

	"golang.org/x/oauth2"
)

type TokenProvider interface {
	Token(context.Context) (*oauth2.Token, error)
}

type Client struct {
	baseURL    string
	tokens     TokenProvider
	httpClient *http.Client
}

func NewClient(baseURL string, tokens TokenProvider) *Client {
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
	return c.get(ctx, "/api/1/vehicles")
}

func (c *Client) VehicleData(ctx context.Context, vin string) (json.RawMessage, error) {
	if vin == "" {
		return nil, errors.New("VIN is required")
	}
	for _, char := range vin {
		if !(char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9') {
			return nil, errors.New("VIN must contain only letters and digits")
		}
	}
	return c.get(ctx, "/api/1/vehicles/"+vin+"/vehicle_data")
}

func (c *Client) get(ctx context.Context, path string) (json.RawMessage, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.Path != "" || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("TESLA_AUDIENCE must be a Fleet API HTTPS base URL")
	}
	if c.tokens == nil {
		return nil, errors.New("Tesla token provider is not configured")
	}
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("get Tesla token: %w", err)
	}
	if !token.Valid() {
		return nil, errors.New("Tesla token has expired or is empty; sign in through /auth/tsla again")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	token.SetAuthHeader(request)
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Tesla Fleet API: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("Tesla rejected authorization; reconnect through /auth/tsla")
	}
	const maxResponseBytes = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if response.StatusCode != http.StatusOK {
		// Keep the HTTP status even when Tesla's error body cannot be read.
		if err != nil || len(body) > maxResponseBytes {
			body = nil
		}
		return nil, fleetAPIError(response.StatusCode, body, c.baseURL)
	}
	if err != nil {
		return nil, fmt.Errorf("read Tesla response: %w", err)
	}
	if len(body) > maxResponseBytes || !json.Valid(body) {
		return nil, errors.New("Tesla returned an invalid or oversized JSON response")
	}
	return json.RawMessage(body), nil
}

func fleetAPIError(status int, body []byte, audience string) error {
	message := fmt.Sprintf("Tesla request failed: HTTP %d", status)
	// Expose diagnostic fields rather than the entire upstream response.
	var details struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		TransactionID    string `json:"txid"`
	}
	if json.Unmarshal(body, &details) == nil {
		if details.Error != "" {
			message += fmt.Sprintf("; Tesla error: %.1024q", details.Error)
		}
		if details.ErrorDescription != "" {
			message += fmt.Sprintf("; description: %.1024q", details.ErrorDescription)
		}
		if details.TransactionID != "" {
			message += fmt.Sprintf("; txid: %.128q", details.TransactionID)
		}
	}
	if status == http.StatusPreconditionFailed {
		message += fmt.Sprintf("; check partner account registration in region %s: https://developer.tesla.com/docs/fleet-api/endpoints/partner-endpoints#register", audience)
	}
	return errors.New(message)
}
