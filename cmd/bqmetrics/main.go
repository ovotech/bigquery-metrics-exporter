package main

import (
	"context"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/daemon"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const CmdName = "bqmetrics"

func main() {
	cfg, err := config.NewConfig(CmdName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse config")
	}

	ctx := context.Background()
	app, err := daemon.NewRunner(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create runner")
	}

	if err = app.RunOnce(ctx); err != nil {
		log.Fatal().Err(err).Msg("Error during run")
	}
}

func init() {
	log.Logger = log.Output(zerolog.NewConsoleWriter())
	ll := config.GetEnv("LOG_LEVEL", "info")
	level, err := zerolog.ParseLevel(ll)
	if err != nil {
		log.Error().Msgf("Error parsing LOG_LEVEL with value %s", ll)
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Info().Msgf("Logging level set to %s", zerolog.GlobalLevel())
}
