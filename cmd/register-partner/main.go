package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"
	"tsla-remote-mcp/internal/auth"
	"tsla-remote-mcp/internal/tesla"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func main() {
	verifyOnly := flag.Bool("verify-only", false, "Verify the existing registration without registering again")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	config := &clientcredentials.Config{
		ClientID: os.Getenv("TESLA_CLIENT_ID"), ClientSecret: os.Getenv("TESLA_CLIENT_SECRET"),
		TokenURL: auth.TeslaTokenURL, AuthStyle: oauth2.AuthStyleInParams,
		Scopes:         []string{"openid", "vehicle_device_data", "vehicle_cmds", "vehicle_charging_cmds"},
		EndpointParams: url.Values{"audience": {os.Getenv("TESLA_AUDIENCE")}},
	}
	operation := tesla.RegisterPartner
	if *verifyOnly {
		operation = tesla.VerifyPartner
	}
	if err := operation(ctx, config, os.Getenv("DOMAIN_SERVICE")); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Tesla partner registration confirmed. Retry list_vehicles using the connected Tesla account.")
}
