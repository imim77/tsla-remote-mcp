package auth

import (
	"os"
	"strings"

	"golang.org/x/oauth2"
)

const (
	CallbackURL           = "/auth/callback"
	TeslaAuthorizationURL = "https://auth.tesla.com/oauth2/v3/authorize"
	TeslaTokenURL         = "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/token"
)

func InitializeOAuthConfig() *oauth2.Config {
	domain := strings.TrimRight(os.Getenv("DOMAIN_SERVICE"), "/")
	if domain == "" {
		domain = "http://localhost:8080"
	}
	return &oauth2.Config{
		ClientID:     os.Getenv("TESLA_CLIENT_ID"),
		ClientSecret: os.Getenv("TESLA_CLIENT_SECRET"),
		RedirectURL:  domain + CallbackURL,
		Scopes:       []string{"openid", "offline_access", "vehicle_device_data", "vehicle_cmds", "vehicle_charging_cmds"},
		Endpoint: oauth2.Endpoint{
			AuthURL:   TeslaAuthorizationURL,
			TokenURL:  TeslaTokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}
