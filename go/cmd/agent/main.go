package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/collector/snapshot"
	"github.com/kis-open-api/go/internal/domesticfutureoption"
	"github.com/kis-open-api/go/internal/domesticstock"
	"github.com/kis-open-api/go/internal/external/kofia"
	"github.com/kis-open-api/go/internal/external/naver"
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
	if len(args) < 2 || args[0] != "report" {
		return fmt.Errorf("usage: go run ./cmd/agent report <market-snapshot|credit-balance> [args...]")
	}
	switch args[1] {
	case "market-snapshot":
		return runMarketSnapshot(args[2:])
	case "credit-balance":
		return runCreditBalance(args[2:])
	default:
		return fmt.Errorf("unknown report type %q (supported: market-snapshot, credit-balance)", args[1])
	}
}

func runMarketSnapshot(args []string) error {
	opts, err := parseOptions(args)
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
	naverClient := naver.NewClient(client.Client, os.Getenv("USER_AGENT"))
	kofiaClient := kofia.NewCachedClient(envDefault("KOFIA_CACHE_DIR", ".cache"), os.Getenv("USER_AGENT"))
	result := snapshot.Collect(ctx, snapshot.Deps{
		DomesticStock:  domesticstock.NewService(client),
		DomesticFuture: domesticfutureoption.NewService(client),
		Yahoo:          yahooClient,
		Naver:          naverClient,
		KOFIA:          kofiaClient,
	}, opts)
	// ── 이전 날짜 스냅샷 로드 (비교용) ──────────────────────────────────────────
	outDir := envDefault("SNAPSHOT_OUTPUT_DIR", ".cache/snapshots")
	date := time.Now().Format("20060102")
	if opts.Date != "" {
		normalized := strings.ReplaceAll(opts.Date, "-", "")
		if len(normalized) == 8 {
			date = normalized
		}
	}
	var prev *snapshot.SnapshotJSON
	if outDir != "" {
		if p, err := snapshot.LoadPreviousSnapshot(outDir, date); err == nil && p != nil {
			prev = p
			fmt.Fprintf(os.Stderr, "[snapshot] 비교 데이터 로드: %s\n", p.Date)
		}
	}

	// ── 렌더링 (이전 데이터가 있으면 비교 표시) ──────────────────────────────────
	output := snapshot.Render(result, prev)
	fmt.Print(output)

	// ── JSON + Markdown 저장 ─────────────────────────────────────────────────
	if outDir != "" {
		// JSON 저장 (다음 날 비교용)
		if jsonPath, err := snapshot.SaveJSON(result, outDir); err == nil {
			fmt.Fprintf(os.Stderr, "[snapshot] JSON saved: %s\n", jsonPath)
		} else {
			fmt.Fprintf(os.Stderr, "[snapshot] JSON save failed: %v\n", err)
		}
		// Markdown 저장 (사람 읽기용)
		mdPath := filepath.Join(outDir, fmt.Sprintf("market_snapshot.%s.md", date))
		if err := os.WriteFile(mdPath, []byte(output), 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "[snapshot] MD saved: %s\n", mdPath)
		} else {
			fmt.Fprintf(os.Stderr, "[snapshot] MD save failed: %v\n", err)
		}
	}
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
