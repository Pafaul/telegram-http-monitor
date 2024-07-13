package main

import (
	"context"
	"fmt"
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
		ClientId: c.Sender().ID,
	}

	err := _q.InsertRequest(context.Background(), monitor_db.InsertRequestParams{
		Clientid: request.ClientId,
		Endpoint: request.Endpoint,
	})
	if err != nil {
		log.Error().
			Int64("clientId", request.ClientId).
			Err(err).
			Msg("add request")
	}

	if _httpMonitor.RequestExists(request) {
		return c.Send(fmt.Sprintf("url %s is already being monitored", urlToAdd))
	}

	_httpMonitor.AddRequest(request)

	return c.Send(fmt.Sprintf("Endpoint %s added to monitoring", urlToAdd))
}

func listMonitoredEndpoints(c tele.Context) error {
	clientId := c.Sender().ID
	dbRequests, err := _q.GetRequestsByClientId(context.Background(), clientId)
	if err != nil {
		log.Error().Int64("clientId", clientId).Err(err).Msg("list requests")
		return nil
	}

	clientRequests := make([]EndpointRequest, len(dbRequests))
	for i, r := range dbRequests {
		clientRequests[i] = EndpointRequest{Endpoint: r.Endpoint, ClientId: clientId}
	}

	if len(clientRequests) == 0 {
		return c.Send("client has no active requests")
	}

	clientMsg := "endpoints:\n"
	for id, r := range clientRequests {
		clientMsg += fmt.Sprintf("  %2d. %s\n", id+1, r.Endpoint)
	}

	return c.Send(clientMsg)
}

func removeMonitoredEndpoint(c tele.Context) error {
	if len(c.Args()) != 1 {
		return c.Send("usage: /rm endpoint_to_remove or /rm endpoint_index_in_list")
	}

	urlToRemove := c.Args()[0]
	clientId := c.Sender().ID

	if numeric, err := strconv.ParseInt(urlToRemove, 10, 64); err == nil {
		amountOfEndpoints, queryErr := _q.GetClientEndpointsAmount(context.Background(), clientId)
		if queryErr != nil {
			log.Error().
				Err(err).
				Int64("clientId", clientId).
				Msg("remove monitored endpoint, get endpoint amount")
			return c.Send(fmt.Sprintf("could not load your endpoints: %s", err))
		}
		if numeric == 0 {
			return c.Send("Numeric id cannot be lower than 1")
		}
		if numeric > amountOfEndpoints {
			return c.Send(fmt.Sprintf("Invalid id provided, input a number from 1 to %d", amountOfEndpoints))
		}

		endpointByIndex, queryErr := _q.GetClientEndpointByIndex(context.Background(), monitor_db.GetClientEndpointByIndexParams{
			Clientid: clientId,
			Offset:   numeric - 1,
		})
		if queryErr != nil {
			log.Error().
				Err(err).
				Int64("clientId", clientId).
				Int64("numeric", numeric).
				Msg("remove monitored endpoint, get endpoint by index")
		}
		urlToRemove = endpointByIndex.Endpoint
	}

	if _, err := url.ParseRequestURI(urlToRemove); err != nil {
		return c.Send(fmt.Sprintf("error parsing provided uri: %s", err.Error()))
	}

	request := &EndpointRequest{Endpoint: urlToRemove, ClientId: clientId}

	err := _q.RemoveRequest(context.Background(), monitor_db.RemoveRequestParams{
		Clientid: request.ClientId,
		Endpoint: request.Endpoint,
	})
	if err != nil {
		log.Error().
			Int64("clientId", request.ClientId).
			Err(err).
			Msg("remove request")
	}

	removed := _httpMonitor.RemoveRequest(request)
	if !removed {
		return c.Send(fmt.Sprintf("could not find requested endpoint: %s", urlToRemove))
	}

	return c.Send(fmt.Sprintf("removed endpoint: %s", urlToRemove))
}

func SendErrorsToClients(ctx context.Context, bot *tele.Bot, errorChannel <-chan RequestError) {
	for {
		select {
		case <-ctx.Done():
			return
		case requestErr := <-errorChannel:
			_, sendErr := bot.Send(
				&tele.User{ID: requestErr.ClientId},
				fmt.Sprintf(
					"received error: %s\nfor endpoint: %s",
					requestErr.Error.Error(),
					requestErr.Endpoint,
				),
			)
			if sendErr != nil {
				log.Error().
					Int64("client", requestErr.ClientId).
					Str("requestError", requestErr.Error.Error()).
					Err(sendErr).
					Msg("could not send error to client")
			}
		}
	}
}
