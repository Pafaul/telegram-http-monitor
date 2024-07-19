package main

import (
	"context"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	tele "gopkg.in/telebot.v3"
	"os"
	"os/signal"
	"runtime"
	"sync"
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
		runtime.Goexit()
	}
	defer func() {
		err := db.Close()
		if err != nil {
			log.Error().Err(err).Msg("db close")
		}
	}()

	httpMonitor := NewHttpMonitor(config.Monitor.AmountOfWorkers)

	bot, botErr := NewBot(config, httpMonitor, db)
	if botErr != nil {
		log.Error().Err(botErr).Msg("could not start bot")
		runtime.Goexit()
	}

	errorChannel := make(chan RequestError)
	cancel, wg := start(bot, httpMonitor, db, errorChannel)

	sigKill := make(chan os.Signal)
	signal.Notify(sigKill, os.Interrupt, os.Kill)
	go func() {
		select {
		case s := <-sigKill:
			cancel()
			log.Warn().Str("signal", s.String()).Msg("sigkill received")
			log.Info().Msg("stopping bot")
			bot.Stop()
			close(errorChannel)
		}
	}()

	wg.Wait()
}

func start(bot *tele.Bot, httpMonitor *HttpMonitor, db *sql.DB, errorChannel chan RequestError) (context.CancelFunc, *sync.WaitGroup) {
	senderCtx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(3)

	go func(wg *sync.WaitGroup) {
		bot.Start()
		wg.Done()
	}(&wg)

	go func(wg *sync.WaitGroup) {
		httpMonitor.StartMonitor(senderCtx, db, errorChannel)
		wg.Done()
	}(&wg)

	go func(wg *sync.WaitGroup) {
		SendErrorsToClients(senderCtx, bot, errorChannel)
		wg.Done()
	}(&wg)

	return cancel, &wg
}
