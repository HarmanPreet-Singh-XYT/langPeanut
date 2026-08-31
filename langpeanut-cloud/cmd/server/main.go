// cmd/server/main.go — langpeanut-cloud trusted host process.
//
// Starts two things in the same binary:
//  1. HTTP server (API + static Next.js build served by Caddy, but all /api/* routes
//     handled here) — see internal/api.RegisterRoutes.
//  2. In-process worker goroutine — polls the jobs table and executes the
//     localization pipeline per job — see internal/worker.Run.
//
// All configuration comes from environment variables (12-factor style).
// See .env.example for the full variable list.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/langPeanut/langpeanut-cloud/internal/api"
	"github.com/langPeanut/langpeanut-cloud/internal/auth"
	"github.com/langPeanut/langpeanut-cloud/internal/db"
	"github.com/langPeanut/langpeanut-cloud/internal/mirror"
	"github.com/langPeanut/langpeanut-cloud/internal/worker"
)

func main() {
	// ── Logging ──────────────────────────────────────────────────────────────
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// ── Special sub-commands ──────────────────────────────────────────────────
	if len(os.Args) > 1 && os.Args[1] == "keygen" {
		// Usage: langpeanut-cloud keygen
		// Generates a random 32-byte MASTER_KEY and prints it.
		key, err := auth.GenerateMasterKey()
		if err != nil {
			slog.Error("keygen failed", "err", err)
			os.Exit(1)
		}
		fmt.Println(key)
		return
	}

	// ── Configuration from environment ───────────────────────────────────────
	cfg := mustConfig()

	// ── Database ──────────────────────────────────────────────────────────────
	database, err := db.Open(cfg.databasePath)
	if err != nil {
		slog.Error("db open", "err", err)
		os.Exit(1)
	}
	defer database.Close()
	slog.Info("db ready", "path", cfg.databasePath)

	// ── Mirror manager ────────────────────────────────────────────────────────
	mgr, err := mirror.New(cfg.mirrorsDir)
	if err != nil {
		slog.Error("mirror manager", "err", err)
		os.Exit(1)
	}

	// ── HTTP server ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()
	h := &api.Handler{
		DB:                database,
		MasterKey:         cfg.masterKey,
		SessionSecret:     cfg.sessionSecret,
		AppID:             cfg.appID,
		PrivateKeyPEM:     cfg.privateKeyPEM,
		OAuthClientID:     cfg.oauthClientID,
		OAuthClientSecret: cfg.oauthClientSecret,
		WebhookSecret:     cfg.webhookSecret,
		PublicBaseURL:     cfg.publicBaseURL,
	}
	api.RegisterRoutes(mux, h)

	// Serve static web build if present
	webDir := getenv("WEB_DIR", "/app/web/out")
	if _, err := os.Stat(webDir); err == nil {
		slog.Info("serving static web assets", "dir", webDir)
		mux.Handle("/", spaHandler(webDir))
	} else if _, err := os.Stat("./web/out"); err == nil {
		slog.Info("serving local static web assets", "dir", "./web/out")
		mux.Handle("/", spaHandler("./web/out"))
	}

	srv := &http.Server{
		Addr:         cfg.listenAddr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// ── Graceful shutdown context ─────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── Worker goroutine ──────────────────────────────────────────────────────
	workerCfg := worker.Config{
		DB:            database,
		Mirror:        mgr,
		JobsDir:       cfg.jobsDir,
		MasterKey:     cfg.masterKey,
		RunnerImage:   cfg.runnerImage,
		AppID:         cfg.appID,
		PrivateKeyPEM: cfg.privateKeyPEM,
	}
	go worker.Run(ctx, workerCfg)

	// ── HTTP server goroutine ─────────────────────────────────────────────────
	go func() {
		slog.Info("http server listening", "addr", cfg.listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server", "err", err)
			stop()
		}
	}()

	// Wait for shutdown signal.
	<-ctx.Done()
	slog.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown", "err", err)
	}
	slog.Info("shutdown complete")
}

// ─── Config ──────────────────────────────────────────────────────────────────

type serverConfig struct {
	listenAddr        string
	databasePath      string
	mirrorsDir        string
	jobsDir           string
	masterKey         string
	sessionSecret     string
	runnerImage       string
	appID             string // GitHub App ID as a string (AppConfig.AppID is a string)
	privateKeyPEM     []byte
	oauthClientID     string
	oauthClientSecret string
	webhookSecret     string
	publicBaseURL     string
}

func mustConfig() serverConfig {
	cfg := serverConfig{
		listenAddr:        getenv("LISTEN_ADDR", ":8080"),
		databasePath:      getenv("DATABASE_PATH", "/data/langpeanut.db"),
		mirrorsDir:        getenv("MIRRORS_DIR", "/data/mirrors"),
		jobsDir:           getenv("JOBS_DIR", "/data/jobs"),
		masterKey:         mustGetenv("MASTER_KEY"),
		sessionSecret:     getenv("SESSION_SECRET", ""),
		runnerImage:       getenv("RUNNER_IMAGE", "langpeanut-runner:latest"),
		appID:             mustGetenv("GITHUB_APP_ID"),
		oauthClientID:     mustGetenv("GITHUB_APP_CLIENT_ID"),
		oauthClientSecret: mustGetenv("GITHUB_APP_CLIENT_SECRET"),
		webhookSecret:     mustGetenv("GITHUB_WEBHOOK_SECRET"),
		publicBaseURL:     getenv("PUBLIC_BASE_URL", "http://localhost:8080"),
	}
	if cfg.sessionSecret == "" {
		// Fall back to MASTER_KEY so a single generated key covers both purposes
		// in simple deployments; operators can set SESSION_SECRET separately
		// to rotate sessions without re-encrypting stored credentials.
		cfg.sessionSecret = cfg.masterKey
	}

	pemPath := mustGetenv("GITHUB_APP_PRIVATE_KEY_PATH")
	pem, err := os.ReadFile(pemPath)
	if err != nil {
		slog.Error("read private key", "path", pemPath, "err", err)
		os.Exit(1)
	}
	cfg.privateKeyPEM = pem

	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "var", key)
		os.Exit(1)
	}
	return v
}

func spaHandler(webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cleanPath := filepath.Clean(r.URL.Path)
		targetPath := filepath.Join(webDir, cleanPath)

		// 1. Direct file match (e.g. static assets, css, js, images)
		if fi, err := os.Stat(targetPath); err == nil && !fi.IsDir() {
			http.ServeFile(w, r, targetPath)
			return
		}

		// 2. Directory with index.html (e.g. /login/ -> /app/web/out/login/index.html)
		indexPath := filepath.Join(targetPath, "index.html")
		if fi, err := os.Stat(indexPath); err == nil && !fi.IsDir() {
			http.ServeFile(w, r, indexPath)
			return
		}

		// 3. HTML extension match (e.g. /login -> /app/web/out/login.html)
		htmlPath := targetPath + ".html"
		if fi, err := os.Stat(htmlPath); err == nil && !fi.IsDir() {
			http.ServeFile(w, r, htmlPath)
			return
		}

		// 4. Root index.html fallback for client-side routing
		rootIndex := filepath.Join(webDir, "index.html")
		if fi, err := os.Stat(rootIndex); err == nil && !fi.IsDir() {
			http.ServeFile(w, r, rootIndex)
			return
		}

		http.NotFound(w, r)
	}
}
