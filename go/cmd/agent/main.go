package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/collector/snapshot"
	"github.com/kis-open-api/go/internal/domesticfutureoption"
	"github.com/kis-open-api/go/internal/domesticstock"
	"github.com/kis-open-api/go/internal/external/yahoo"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	_ = godotenv.Load()
	if len(args) < 2 || args[0] != "report" || args[1] != "market-snapshot" {
		return fmt.Errorf("usage: go run ./cmd/agent report market-snapshot [--date YYYY-MM-DD|YYYYMMDD]")
	}

	opts, err := parseOptions(args[2:])
	if err != nil {
		return err
	}
	client, err := newKISClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := client.EnsureAuthToken(ctx); err != nil {
		return fmt.Errorf("auth token: %w", err)
	}

	yahooClient := yahoo.NewClient(client.Client, yahoo.Config{UserAgent: os.Getenv("USER_AGENT")})
	result := snapshot.Collect(ctx, snapshot.Deps{
		DomesticStock:  domesticstock.NewService(client),
		DomesticFuture: domesticfutureoption.NewService(client),
		Yahoo:          yahooClient,
	}, opts)
	fmt.Print(snapshot.Render(result))
	return nil
}

func newKISClient() (*auth.KIClient, error) {
	appKey, appSecret := strings.TrimSpace(os.Getenv("APP_KEY")), strings.TrimSpace(os.Getenv("APP_SECRET"))
	if appKey == "" || appSecret == "" {
		return nil, fmt.Errorf("APP_KEY and APP_SECRET are required")
	}
	client := auth.NewKIClient(appKey, appSecret, "https://openapi.koreainvestment.com:9443", os.Getenv("USER_AGENT"))
	client.Client = &http.Client{Timeout: 15 * time.Second}
	client.SetTokenCachePath(envDefault("AUTH_TOKEN_FILE", ".auth_token.json"))
	return client, nil
}
