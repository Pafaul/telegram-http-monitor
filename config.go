package main

import (
	"bytes"
	"database/sql"
	"errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
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
