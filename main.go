package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"os/signal"
)

type (
	Config struct {
		Token   string `yaml:"token"`
		Monitor struct {
			AmountOfWorkers int `yaml:"amountOfWorkers"`
		} `yaml:"monitor"`
	}
)

var (
	httpMonitor  *HttpMonitor
	errorChannel = make(chan RequestError)
)

func main() {
	config, err := loadConfig("./config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("config loading error")
		os.Exit(1)
	}

	setupLogger(config)

	httpMonitor = NewHttpMonitor(config.Monitor.AmountOfWorkers, errorChannel)

	bot, botErr := NewBot(config)
	if botErr != nil {
		log.Error().Err(botErr).Msg("could not start bot")
		os.Exit(1)
	}

	sigKill := make(chan os.Signal, 1)
	signal.Notify(sigKill, os.Interrupt)

	blocker := make(chan int)

	senderCtx, senderCancel := context.WithCancel(context.Background())

	go bot.Start()
	go httpMonitor.StartMonitor()
	go SendErrorsToClients(senderCtx, bot, errorChannel)

	go func() {
		select {
		case s := <-sigKill:
			senderCancel()
			log.Warn().Str("signal", s.String()).Msg("sigkill received")
			log.Info().Msg("stopping bot")
			bot.Stop()
			log.Info().Msg("bot is stopped")

			log.Info().Msg("stopping http monitor")
			httpMonitor.StopMonitor()
			close(errorChannel)
			log.Info().Msg("http monitor is stopped")

			blocker <- 1
		}
	}()

	<-blocker
}

func loadConfig(configFile string) (*Config, error) {
	configFileContent, err := os.ReadFile(configFile)
	if err != nil {
		log.Error().Err(err).Msg("read config file")
		return nil, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(configFileContent))
	decoder.KnownFields(true)
	config := new(Config)

	decoderErr := decoder.Decode(config)
	if decoderErr != nil {
		log.Error().Err(decoderErr).Msg("decoding config file")
		return nil, decoderErr
	}

	if len(config.Token) == 0 {
		return nil, errors.New("token in config file is missing")
	}

	if config.Monitor.AmountOfWorkers == 0 {
		config.Monitor.AmountOfWorkers = 1
	}

	return config, nil
}

func setupLogger(_ *Config) {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		Level(zerolog.DebugLevel).
		With().
		Timestamp().
		Logger()
}
