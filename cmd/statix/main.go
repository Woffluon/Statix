package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/statix/statix/internal/auth"
	"github.com/statix/statix/internal/config"
	"github.com/statix/statix/internal/metrics"
	"github.com/statix/statix/internal/tlsmanager"
	"github.com/statix/statix/internal/webui"
)

var version = "dev"

func bindListener(listenAddr string, allowFallback bool, maxTries int) (net.Listener, string, bool, error) {
	host, portStr, err := net.SplitHostPort(listenAddr)
	if err != nil {
		if len(listenAddr) > 0 && listenAddr[0] == ':' {
			host = ""
			portStr = listenAddr[1:]
		} else {
			ln, err := net.Listen("tcp", listenAddr)
			if err != nil {
				return nil, "", false, err
			}
			return ln, listenAddr, false, nil
		}
	}

	startPort, err := strconv.Atoi(portStr)
	if err != nil {
		ln, err := net.Listen("tcp", listenAddr)
		if err != nil {
			return nil, "", false, err
		}
		return ln, listenAddr, false, nil
	}

	var lastErr error
	tries := 1
	if allowFallback {
		tries = maxTries
	}

	for i := 0; i < tries; i++ {
		targetPort := startPort + i
		targetAddr := fmt.Sprintf("%s:%d", host, targetPort)
		ln, err := net.Listen("tcp", targetAddr)
		if err == nil {
			return ln, targetAddr, i > 0, nil
		}
		lastErr = err
	}

	return nil, "", false, fmt.Errorf("failed to bind on %s (tried %d ports): %w", listenAddr, tries, lastErr)
}

func main() {
	configPathFlag := flag.String("config", "/etc/statix/config.yaml", "path to config file")
	devFlag := flag.Bool("dev", false, "enable development log format")
	flag.Parse()

	configPath := *configPathFlag

	// Logger initialization
	var logger *slog.Logger
	if *devFlag {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	logger.Info("starting statix", "version", version, "config_path", configPath)

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Info("config file not found, starting in setup wizard mode", "path", configPath)
			cfg = config.DefaultConfig()
			_ = os.MkdirAll(filepath.Dir(configPath), 0700)
		} else {
			logger.Error("failed to load configuration", "error", err)
			os.Exit(1)
		}
	}

	// Bind TCP listener cleanly with port fallback if enabled
	ln, actualAddr, fallbackUsed, err := bindListener(cfg.ListenAddr, cfg.IsPortFallbackEnabled(), 10)
	if err != nil {
		logger.Error("failed to bind TCP listener", "requested_addr", cfg.ListenAddr, "error", err)
		os.Exit(1)
	}

	if fallbackUsed {
		logger.Warn("requested port was busy; dynamically bound to fallback port",
			"requested_addr", cfg.ListenAddr,
			"bound_addr", actualAddr,
		)
		cfg.ListenAddr = actualAddr
	} else {
		logger.Info("http server socket bound successfully", "addr", actualAddr)
	}

	// Override log format if specified in env/flag
	if os.Getenv("LOG_FORMAT") == "text" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	// Dependency wiring
	sessionTTL := 24 * time.Hour
	store := auth.NewSessionStore(sessionTTL)
	rateLimiter := auth.NewRateLimiter(5, 15*time.Minute, 15*time.Minute)

	// Calculate ring buffer capacity: ceil(historyDuration / interval)
	collectInterval := time.Duration(cfg.CollectIntervalSeconds) * time.Second
	historyDuration := time.Duration(cfg.HistoryDurationHours) * time.Hour
	capacity := int(historyDuration / collectInterval)
	if capacity <= 0 {
		capacity = 10800
	}

	buffer := metrics.NewRingBuffer(capacity)
	collector := metrics.New(metrics.CollectorConfig{
		Interval:     collectInterval,
		TopProcesses: 15,
		ProcRoot:     "/proc",
	}, buffer)

	tlsMgr := tlsmanager.New(cfg, configPath, logger)

	server, err := webui.New(webui.ServerDeps{
		Config:     cfg,
		ConfigPath: configPath,
		Store:      store,
		RateLimit:  rateLimiter,
		Buffer:     buffer,
		Collector:  collector,
		TLS:        tlsMgr,
		Logger:     logger,
	})
	if err != nil {
		_ = ln.Close()
		logger.Error("failed to initialize web server", "error", err)
		os.Exit(1)
	}

	// Context with cancel for background worker lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start collector
	go func() {
		if err := collector.Run(ctx); err != nil {
			logger.Error("collector error", "error", err)
		}
	}()

	// Start server background tasks
	server.Start(ctx)

	// HTTP Server configuration
	httpServer := &http.Server{
		Addr:         actualAddr,
		Handler:      server,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Signal handling
	sigCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	go func() {
		logger.Info("http server listening", "addr", actualAddr)
		if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server listening error", "error", err)
			os.Exit(1)
		}
	}()

	<-sigCtx.Done()
	logger.Info("shutting down statix gracefully...")

	// 30 second timeout context for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http server forced to shutdown", "error", err)
		os.Exit(1)
	}

	cancel() // Stop collector & background goroutines
	logger.Info("statix shutdown complete")
}
