package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/International-Combat-Archery-Alliance/mailing-list-api/api"
	"github.com/International-Combat-Archery-Alliance/telemetry"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"
)

var tracer = otel.Tracer("github.com/International-Combat-Archery-Alliance/mailing-list-api/cmd")

const mailingListGroupName = "ICAA Mailing List"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	logger.Info("starting up")
	if err := run(logger); err != nil {
		logger.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	mailingListAPI, traceShutdown, err := setupAPI(logger)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := traceShutdown(shutdownCtx); err != nil {
			logger.Error("failed to shutdown telemetry", "error", err)
		}
	}()
	if err != nil {
		return err
	}

	serverSettings := getServerSettingsFromEnv()

	sigCtx, sigStop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer sigStop()

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- mailingListAPI.ListenAndServe(serverSettings.Host, serverSettings.Port)
	}()

	select {
	case <-sigCtx.Done():
		logger.Info("shutting down gracefully")
		return nil
	case err := <-serverErrCh:
		if err != nil {
			logger.Error("error running server", "error", err)
			return err
		}
		return nil
	}
}

func setupAPI(logger *slog.Logger) (*api.API, func(context.Context) error, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	env := getAPIEnvironment()

	licenseKey, err := getNewRelicLicenseKey(ctx, env)
	if err != nil {
		return nil, func(context.Context) error { return nil }, fmt.Errorf("new relic license key: %w", err)
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "otlp.nr-data.net:4317"
	}

	traceShutdown, flushTraces, err := telemetry.Init(ctx, telemetry.Options{
		ServiceName: "mailing-list-api",
		Endpoint:    endpoint,
		APIKey:      licenseKey,
		Lambda:      telemetry.LambdaInfoFromEnv(),
	})
	if err != nil {
		return nil, traceShutdown, fmt.Errorf("telemetry init: %w", err)
	}

	ctx, startupSpan := tracer.Start(ctx, "startup")
	defer startupSpan.End()

	var (
		cfg            AppConfig
		httpClient     *http.Client
	)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		ctx, span := tracer.Start(gCtx, "init-config")
		defer span.End()

		var err error
		cfg, err = fetchAppConfig(ctx, env)
		if err != nil {
			span.RecordError(err)
		}
		return err
	})

	g.Go(func() error {
		_, span := tracer.Start(gCtx, "init-http-client")
		defer span.End()

		httpClient = &http.Client{Timeout: 30 * time.Second}
		return nil
	})

	if err := g.Wait(); err != nil {
		startupSpan.RecordError(err)
		return nil, traceShutdown, err
	}

	subscriberManager := createSubscriberManager(logger, env, cfg.MailerLiteAPIKey)

	turnstileValidator, err := createTurnstileValidator(httpClient, cfg.TurnstileSecretKey)
	if err != nil {
		return nil, traceShutdown, fmt.Errorf("create turnstile validator: %w", err)
	}

	mailingListGroupID, err := subscriberManager.FindOrCreateGroup(ctx, mailingListGroupName)
	if err != nil {
		return nil, traceShutdown, fmt.Errorf("find or create mailing list group: %w", err)
	}
	logger.Info("mailing list group ready", slog.String("groupID", mailingListGroupID))

	mailingListAPI := api.NewAPI(subscriberManager, turnstileValidator, mailingListGroupID, logger, env, flushTraces)

	return mailingListAPI, traceShutdown, nil
}

type ServerSettings struct {
	Host string
	Port string
}

func getServerSettingsFromEnv() ServerSettings {
	return ServerSettings{
		Host: getEnvOrDefault("HOST", "0.0.0.0"),
		Port: getEnvOrDefault("PORT", "8080"),
	}
}
