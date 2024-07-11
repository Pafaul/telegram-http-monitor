package main

import (
	"bytes"
	"errors"
	"fmt"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/yaml.v3"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"
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

func loadConfig(configFile string) (*Config, error) {
	configFileContent, err := os.ReadFile(configFile)
	if err != nil {
		slog.Error("read config file", "err", err.Error())
		return nil, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(configFileContent))
	decoder.KnownFields(true)
	config := new(Config)

	decoderErr := decoder.Decode(config)
	if decoderErr != nil {
		slog.Error("decoding config file", "err", decoderErr.Error())
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

func main() {
	config, err := loadConfig("./config.yaml")
	if err != nil {
		slog.Error("config loading error", "err", err.Error())
		os.Exit(1)
	}

	httpMonitor := NewHttpMonitor(config.Monitor.AmountOfWorkers, errorChannel)

	botSettings := tele.Settings{
		Token:  config.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, botErr := tele.NewBot(botSettings)
	if botErr != nil {
		slog.Error("start bot", "err", botErr.Error())
	}

	commands := []tele.Command{
		{Text: "/start", Description: "Start bot"},
		{Text: "/add", Description: "Add endpoint to monitoring"},
		{Text: "/remove", Description: "Remove endpoint from monitoring"},
		{Text: "/test", Description: "Test if endpoint is active"},
		{Text: "/unsubscribe", Description: "Remove all endpoints from monitoring"},
		{Text: "/help", Description: "Show help message"},
	}
	setCommandsErr := bot.SetCommands(commands)
	if setCommandsErr != nil {
		slog.Error("set bot commands", "err", setCommandsErr.Error())
		bot.Stop()
		os.Exit(1)
	}

	bot.Handle("/start", func(c tele.Context) error {
		return SendHelp(c)
	})
	bot.Handle("/add", func(c tele.Context) error {
		return AddEndpointToMonitor(c)
	})
	bot.Handle("/remove", func(c tele.Context) error {
		return SendHelp(c)
	})
	bot.Handle("/test", func(c tele.Context) error {
		return SendHelp(c)
	})
	bot.Handle("/unsubscribe", func(c tele.Context) error {
		return SendHelp(c)
	})
	bot.Handle("/help", func(c tele.Context) error {
		return SendHelp(c)
	})

	sigKill := make(chan os.Signal, 1)
	signal.Notify(sigKill, os.Interrupt)

	blocker := make(chan int)

	go func() {
		slog.Info("stating bot")
		bot.Start()
	}()

	go func() {
		select {
		case s := <-sigKill:
			slog.Warn("sigkill received", "signal", s)
			slog.Info("stopping bot")
			bot.Stop()
			slog.Info("bot is stopped")

			slog.Info("stopping http monitor")
			httpMonitor.StopMonitor()
			close(errorChannel)
			slog.Info("http monitor is stopped")

			blocker <- 1
		}
	}()

	<-blocker
}

func SendHelp(c tele.Context) error {
	msg := `
available commands: 
	idk man
`
	return c.Send(msg)
}

func AddEndpointToMonitor(c tele.Context) error {
	if len(c.Args()) != 1 {
		return c.Send("usage: /add https://endpoint.com")
	}

	urlToAdd := c.Args()[0]
	_, urlErr := url.ParseRequestURI(urlToAdd)
	if urlErr != nil {
		return c.Send(urlErr.Error())
	}

	if !strings.HasPrefix(urlToAdd, "http") {
		return c.Send(fmt.Sprintf("url %s is not http/https endpoint", urlToAdd))
	}

	request := EndpointRequest{
		Endpoint: urlToAdd,
		ClientId: c.Sender().ID,
	}

	if httpMonitor.RequestExists(request) {
		return c.Send(fmt.Sprintf("url %s is already being monitored", urlToAdd))
	}

	httpMonitor.AddRequest(request)

	return c.Send(fmt.Sprintf("Endpoint %s added to monitoring", urlToAdd))
}
