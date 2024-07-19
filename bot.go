package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	tele "gopkg.in/telebot.v3"
	"net/url"
	"pafaul/telegram-http-monitor/monitor_db"
	"strconv"
	"strings"
	"time"
)

type (
	BotFunction struct {
		command tele.Command
		handler tele.HandlerFunc
	}
)

var (
	botSetup = []BotFunction{
		{
			command: tele.Command{Text: "/start", Description: "Start bot"},
			handler: startMsg,
		},
		{
			command: tele.Command{Text: "/add", Description: "Add endpoint to monitoring"},
			handler: addEndpointToMonitor,
		},
		{
			command: tele.Command{Text: "/rm", Description: "Remove endpoint from monitoring"},
			handler: removeMonitoredEndpoint,
		},
		{
			command: tele.Command{Text: "/list", Description: "List endpoints that are monitored"},
			handler: listMonitoredEndpoints,
		},
		{
			command: tele.Command{Text: "/help", Description: "Show help message"},
			handler: sendHelp,
		},
	}
)

var (
	_httpMonitor *HttpMonitor
	_q           *monitor_db.Queries
)

func NewBot(config *Config, monitor *HttpMonitor, q *monitor_db.Queries) (*tele.Bot, error) {
	botSettings := tele.Settings{
		Token:  config.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, botErr := tele.NewBot(botSettings)
	if botErr != nil {
		log.Error().Err(botErr).Msg("start bot")
		return nil, botErr
	}

	setCommandsErr := setupBotCommands(bot)
	if setCommandsErr != nil {
		log.Error().Err(setCommandsErr).Msg("set bot commands")
		bot.Stop()
		return nil, botErr
	}

	setupBotMiddleware(bot)
	setupBotHandlers(bot)

	_httpMonitor = monitor
	_q = q

	return bot, nil
}

func setupBotCommands(bot *tele.Bot) error {
	botCommands := make([]tele.Command, len(botSetup))
	for id, bf := range botSetup {
		botCommands[id] = bf.command
	}
	return bot.SetCommands(botCommands)
}

func setupBotMiddleware(_ *tele.Bot) {
	return
}

func setupBotHandlers(bot *tele.Bot) {
	for _, bf := range botSetup {
		bot.Handle(bf.command.Text, bf.handler)
	}
}

func startMsg(c tele.Context) error {
	msg := `
hurray, you've started the bot!
You can use it to monitor http endpoints that are publicly available.
Check the menu for available commands.
`
	return c.Send(msg)
}

func sendHelp(c tele.Context) error {
	msg := `
available commands: 
	check the menu lol why would you click the <b>help</b> button
`
	return c.Send(msg, tele.ModeHTML)
}

func addEndpointToMonitor(c tele.Context) error {
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

	request := &EndpointRequest{
		Endpoint: urlToAdd,
	}

	err := addClientSubscription(context.Background(), _q, c.Sender().ID, urlToAdd)
	if err != nil {
		if !errors.Is(err, sqlite3.ErrConstraintUnique) {
			return c.Send(fmt.Sprintf("url %s is already being monitored", urlToAdd))
		}
		log.Error().
			Int64("clientId", c.Sender().ID).
			Err(err).
			Msg("add request")
		return c.Send("Internal error")
	}

	_httpMonitor.AddRequest(request)

	return c.Send(fmt.Sprintf("Endpoint %s added to monitoring", urlToAdd))
}

func listMonitoredEndpoints(c tele.Context) error {
	clientId := c.Sender().ID
	monitoredEndpoints, err := _q.GetUserMonitoredEndpoints(context.Background(), clientId)
	if err != nil {
		log.Error().Int64("clientId", clientId).Err(err).Msg("list requests")
		return c.Send("Could not retrieve your monitored endpoints, please try again later")
	}

	if len(monitoredEndpoints) == 0 {
		return c.Send("You don't have any active monitored endpoints")
	}

	clientMsg := "endpoints:\n"
	for id, r := range monitoredEndpoints {
		clientMsg += fmt.Sprintf("  %2d. %s\n", id+1, r)
	}

	return c.Send(clientMsg)
}

func removeMonitoredEndpoint(c tele.Context) error {
	if len(c.Args()) != 1 {
		return c.Send("usage: /rm endpoint_to_remove or /rm endpoint_index_in_list")
	}

	urlToRemove := c.Args()[0]
	clientId := c.Sender().ID

	if index, err := strconv.ParseInt(urlToRemove, 10, 64); err == nil {
		urlToRemove, err = getUrlToRemoveByIndex(context.Background(), _q, clientId, index)
		if err != nil {
			log.Error().
				Err(err).
				Int64("clientId", clientId).
				Msg("remove monitored endpoint, get endpoints")
			return c.Send(fmt.Sprintf("could not remove by index, err: %s", err.Error()))
		}
	}

	err := removeUserSubscription(context.Background(), _q, clientId, urlToRemove)
	if err != nil {
		log.Error().
			Int64("clientId", c.Sender().ID).
			Err(err).
			Msg("remove request")
		return c.Send(fmt.Sprintf("Could not remove your monitored endpoint %s", urlToRemove))
	}

	return c.Send(fmt.Sprintf("removed endpoint: %s", urlToRemove))
}

func SendErrorsToClients(ctx context.Context, bot *tele.Bot, errorChannel <-chan RequestError) {
	for {
		select {
		case <-ctx.Done():
			return
		case requestErr := <-errorChannel:
			usersToNotify, err := _q.GetUsersToNotify(ctx, requestErr.Endpoint)
			if err != nil {
				log.Error().
					Err(err).
					Msg("could not load users from db")
			}

			for _, client := range usersToNotify {
				_, sendErr := bot.Send(
					&tele.User{ID: client},
					fmt.Sprintf(
						"received error: %s\nfor endpoint: %s",
						requestErr.Error.Error(),
						requestErr.Endpoint,
					),
				)
				if sendErr != nil {
					log.Error().
						Int64("client", client).
						Str("requestError", requestErr.Error.Error()).
						Err(sendErr).
						Msg("could not send error to client")
				}
			}
		}
	}
}

func addClientSubscription(ctx context.Context, q *monitor_db.Queries, clientId int64, url string) error {
	err := q.AddClient(ctx, clientId)
	if err != nil {
		return err
	}

	urlId, err := insertEndpointOrGetId(ctx, q, url)
	if err != nil {
		return err
	}

	err = q.AddSubscription(ctx, monitor_db.AddSubscriptionParams{
		Clientid: sql.NullInt64{
			Int64: clientId,
			Valid: true,
		},
		Urlid: sql.NullInt64{
			Int64: urlId,
			Valid: true,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func insertEndpointOrGetId(ctx context.Context, q *monitor_db.Queries, url string) (int64, error) {
	urlId, err := q.GetUrlIdToTrack(ctx, url)
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if err != nil {
		urlId, err = q.AddUrlToTrack(ctx, url)
		if errors.Is(err, sqlite3.ErrConstraintUnique) {
			urlId, err = q.GetUrlIdToTrack(ctx, url)
			if err != nil {
				return 0, err
			}
		}
		if err != nil {
			return 0, err
		}
	}

	return urlId, nil
}

var (
	ErrInvalidNumericRange = errors.New("invalid numeric range")
)

func getUrlToRemoveByIndex(ctx context.Context, q *monitor_db.Queries, clientId, endpointId int64) (string, error) {
	userMonitoredEndpoints, queryErr := q.GetUserMonitoredEndpoints(ctx, clientId)
	if queryErr != nil {
		return "", queryErr
	}

	if endpointId == 0 || endpointId > int64(len(userMonitoredEndpoints)) {
		return "", ErrInvalidNumericRange
	}

	return userMonitoredEndpoints[endpointId-1], nil
}

func removeUserSubscription(ctx context.Context, q *monitor_db.Queries, clientId int64, endpoint string) error {
	urlId, err := q.GetUrlIdToTrack(ctx, endpoint)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	err = q.RemoveSubscription(ctx, monitor_db.RemoveSubscriptionParams{
		Clientid: sql.NullInt64{
			Int64: clientId,
			Valid: true,
		},
		Urlid: sql.NullInt64{
			Int64: urlId,
			Valid: true,
		},
	})
	return err
}
