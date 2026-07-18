package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		healthcheck(os.Args[2:])
		return
	}
	fs := flag.NewFlagSet("notiferry", flag.ExitOnError)
	configPath := fs.String("config", "notiferry.yaml", "configuration file")
	_ = fs.Parse(os.Args[1:])
	c, err := LoadConfig(*configPath)
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	token := telegramBotToken(c)
	if token == "" {
		logger.Error("telegram bot token is required in telegram_bot_token or NOTIFERRY_TELEGRAM_BOT_TOKEN")
		os.Exit(1)
	}
	client := &TelegramClient{
		Token:   token,
		BaseURL: "https://api.telegram.org",
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Validate(ctx); err != nil {
		logger.Error("telegram validation failed", "error", err)
		os.Exit(1)
	}
	app := NewApp(c, client, logger)
	srv := &http.Server{Addr: c.Listen, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	sigctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		for range hup {
			next, e := LoadConfig(*configPath)
			if e != nil {
				logger.Error("configuration reload failed", "error", e)
			} else if next.Listen != app.config.Load().Listen {
				logger.Error("configuration reload rejected", "reason", "listen address changed; restart required")
			} else if telegramBotToken(next) != token {
				logger.Error("configuration reload rejected", "reason", "telegram bot token changed; restart required")
			} else {
				app.Reload(next)
				logger.Info("configuration reloaded")
			}
		}
	}()
	select {
	case <-sigctx.Done():
	case err := <-serverErr:
		logger.Error("server failed", "error", err)
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = srv.Shutdown(shutdown)
		cancel()
		os.Exit(1)
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}

func telegramBotToken(c Config) string {
	if token := os.Getenv("NOTIFERRY_TELEGRAM_BOT_TOKEN"); token != "" {
		return token
	}
	return c.TelegramBotToken
}

func healthcheck(args []string) {
	fs := flag.NewFlagSet("healthcheck", flag.ExitOnError)
	url := fs.String("url", "http://127.0.0.1:8080/health/ready", "readiness URL")
	_ = fs.Parse(args)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *url, nil)
	if err != nil {
		os.Exit(1)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
