package tesla

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const PartnerPublicKeyPath = "/.well-known/appspecific/com.tesla.3p.public-key.pem"

// Keep the registered public key available across deployments.
//
//go:embed public-key.pem
var partnerPublicKey []byte

func PartnerPublicKeyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(partnerPublicKey)
	}
}

// RegisterPartner uses application credentials independently of user tokens.
func RegisterPartner(ctx context.Context, config *clientcredentials.Config, origin string) error {
	if config == nil || config.ClientID == "" || config.ClientSecret == "" {
		return errors.New("TESLA_CLIENT_ID and TESLA_CLIENT_SECRET must be configured")
	}
	origin = strings.TrimRight(origin, "/")
	domain, err := url.Parse(origin)
	if err != nil || !httpsOrigin(domain) {
		return errors.New("DOMAIN_SERVICE must be an HTTPS origin matching Tesla Allowed Origins")
	}
	audience := config.EndpointParams.Get("audience")
	fleet, err := url.Parse(audience)
	if err != nil || !httpsOrigin(fleet) {
		return errors.New("TESLA_AUDIENCE must be a Fleet API HTTPS base URL")
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+PartnerPublicKeyPath, nil)
	if err != nil {
		return err
	}
	key, err := partnerRequest(client, request, audience)
	if err != nil {
		return fmt.Errorf("fetch deployed public key: %w", err)
	}
	if !bytes.Equal(bytes.TrimSpace(key), bytes.TrimSpace(partnerPublicKey)) {
		return errors.New("deployed public key differs from the local key; deploy this version before registering")
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	token, err := config.Token(ctx)
	if err != nil {
		// OAuth error bodies can include sensitive fields; use diagnostic fields.
		var rejected *oauth2.RetrieveError
		if errors.As(err, &rejected) && rejected.Response != nil {
			return fmt.Errorf("get partner token: %w", fleetAPIError(rejected.Response.StatusCode, rejected.Body, audience))
		}
		return errors.New("could not obtain partner token; check credentials and Tesla connectivity")
	}
	body, err := json.Marshal(struct {
		Domain string `json:"domain"`
	}{Domain: domain.Hostname()})
	if err != nil {
		return err
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodPost, audience+"/api/1/partner_accounts", bytes.NewReader(body))
	if err != nil {
		return err
	}
	token.SetAuthHeader(request)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if _, err := partnerRequest(client, request, audience); err != nil {
		return fmt.Errorf("register partner account: %w", err)
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, audience+"/api/1/partner_accounts/public_key?domain="+url.QueryEscape(domain.Hostname()), nil)
	if err != nil {
		return err
	}
	token.SetAuthHeader(request)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	body, err = partnerRequest(client, request, audience)
	if err != nil {
		return fmt.Errorf("verify partner registration: %w", err)
	}
	var registered struct {
		Response struct {
			PublicKey string `json:"public_key"`
		} `json:"response"`
	}
	if json.Unmarshal(body, &registered) != nil || strings.TrimSpace(registered.Response.PublicKey) != strings.TrimSpace(string(partnerPublicKey)) {
		return errors.New("registration submitted but Tesla's registered public key could not be confirmed")
	}
	return nil
}

func httpsOrigin(u *url.URL) bool {
	return u != nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}

func partnerRequest(client *http.Client, request *http.Request, audience string) ([]byte, error) {
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	const maxBytes = 64 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		if err != nil || len(body) > maxBytes {
			body = nil
		}
		return nil, fleetAPIError(response.StatusCode, body, audience)
	}
	if err != nil {
		return nil, err
	}
	if len(body) > maxBytes {
		return nil, errors.New("Tesla partner response is too large")
	}
	return body, nil
}
