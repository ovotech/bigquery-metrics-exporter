package main

import (
	"context"
	"fmt"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/daemon"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/health"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	handleSignals(cancel)

	cfg, err := config.NewConfig(fmt.Sprintf("%s (Version %s)", config.AppName, config.Version))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse config")
	}

	if cfg.Profiler.Enabled {
		addr := fmt.Sprintf("localhost:%d", cfg.Profiler.Port)
		log.Info().Msgf("Running profiler on %s", addr)

		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		go func() {
			log.Err(http.ListenAndServe(addr, mux)).Msg("Shutting down profiler")
		}()
	}

	if cfg.HealthCheck.Enabled {
		addr := fmt.Sprintf("localhost:%d", cfg.HealthCheck.Port)
		log.Info().Msgf("Running healthcheck server on %s", addr)

		healthsrv := health.ServiceStatus{Status: health.Ok}

		mux := http.NewServeMux()
		mux.HandleFunc("/health", healthsrv.Handler)

		go func() {
			log.Err(http.ListenAndServe(addr, mux)).Msg("Shutting down healthcheck server")
		}()
	}

	app, err := daemon.NewRunner(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create runner")
	}

	log.Printf("Starting the metrics collection daemon")
	if err = app.RunUntil(ctx); err != nil {
		log.Fatal().Err(err).Msg("Error during run")
	}
}

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.With().Str("application", config.AppName).Str("version", config.Version).Logger()
	ll := config.GetEnv("LOG_LEVEL", "info")
	level, err := zerolog.ParseLevel(ll)
	if err != nil {
		log.Error().Msgf("Error parsing LOG_LEVEL with value %s", ll)
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Info().Msgf("Logging level set to %s", zerolog.GlobalLevel())
}

func handleSignals(cancel context.CancelFunc) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)

	go func() {
		select {
		case <-c:
			signal.Stop(c)
			cancel()
		}
	}()
}
