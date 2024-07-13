package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/yaml.v3"
	"os"
	"os/signal"
	"pafaul/telegram-http-monitor/monitor_db"
)

type (
	Config struct {
		Token    string `yaml:"token"`
		SqliteDB string `yaml:"sqliteDB"`
		Monitor  struct {
			AmountOfWorkers int `yaml:"amountOfWorkers"`
		} `yaml:"monitor"`
	}
)

var (
	errorChannel = make(chan RequestError)
)

func main() {
	config, err := loadConfig("./config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("config loading error")
		os.Exit(1)
	}

	setupLogger(config)

	db, err := openDBConnection(config)
	if err != nil {
		log.Error().Err(err).Msg("open db error")
	}
	defer db.Close()
	q := monitor_db.New(db)

	httpMonitor := NewHttpMonitor(config.Monitor.AmountOfWorkers, errorChannel)

	bot, botErr := NewBot(config, httpMonitor, q)
	if botErr != nil {
		log.Error().Err(botErr).Msg("could not start bot")
		os.Exit(1)
	}

	cancel := start(bot, httpMonitor, q)

	sigKill := make(chan os.Signal, 1)
	signal.Notify(sigKill, os.Interrupt)
	blocker := make(chan int)
	go func() {
		select {
		case s := <-sigKill:
			cancel()
			log.Warn().Str("signal", s.String()).Msg("sigkill received")
			log.Info().Msg("stopping bot")
			bot.Stop()

			log.Info().Msg("stopping http monitor")
			httpMonitor.StopMonitor()
			close(errorChannel)

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

	if len(config.SqliteDB) == 0 {
		return nil, errors.New("sqlite db file is missing")
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

func openDBConnection(config *Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", config.SqliteDB)
	if err != nil {
		log.Error().Err(err).Msg("connecting to db error")
		return nil, err
	}
	pingErr := db.Ping()
	if pingErr != nil {
		log.Error().Err(err).Msg("sqlite db ping")
		return nil, pingErr
	}

	return db, nil
}

func start(bot *tele.Bot, httpMonitor *HttpMonitor, q *monitor_db.Queries) context.CancelFunc {
	senderCtx, cancel := context.WithCancel(context.Background())

	go bot.Start()
	go httpMonitor.StartMonitor(q)
	go SendErrorsToClients(senderCtx, bot, errorChannel)

	return cancel
}
