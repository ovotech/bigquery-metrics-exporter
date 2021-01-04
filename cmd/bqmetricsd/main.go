package main

import (
	"context"
	"fmt"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/daemon"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
)

const cmdName = "bqmetricsd"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	handleSignals(cancel)

	cfg, err := config.NewConfig(fmt.Sprintf("%s (Version %s)", cmdName, config.Version))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse config")
	}

	if cfg.Profiling {
		addr := "localhost:6060"
		log.Info().Msgf("Running profiler on %s", addr)

		go func() {
			log.Err(http.ListenAndServe(addr, nil)).Msg("Shutting down profiler")
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
	log.Logger = log.With().Str("application", cmdName).Str("version", config.Version).Logger()
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
