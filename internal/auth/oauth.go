package auth

import (
	"os"

	"golang.org/x/oauth2"
)

const (
	CallbackURL           = "http://localhost:8080/auth/callback"
	TeslaAuthorizationURL = "https://auth.tesla.com/oauth2/v3/authorize"
	TeslaTokenURL         = "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/token"
)

func InitializeOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("TESLA_CLIENT_ID"),
		ClientSecret: os.Getenv("TESLA_CLIENT_SECRET"),
		RedirectURL:  CallbackURL,
		Scopes:       []string{"openid", "offline_access", "vehicle_device_data", "vehicle_cmds", "vehicle_charging_cmds"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  TeslaAuthorizationURL,
			TokenURL: TeslaTokenURL,
		},
	}
}
