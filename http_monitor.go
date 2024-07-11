package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"
)

type (
	EndpointRequest struct {
		ClientId int64  `json:"clientId" yaml:"clientId"`
		Endpoint string `json:"endpoint" yaml:"endpoint"`
	}

	RequestError struct {
		EndpointRequest
		Error error `json:"error"`
	}

	HttpMonitor struct {
		cancel          context.CancelFunc
		requests        []EndpointRequest
		amountOfWorkers int
		workerChannel   chan EndpointRequest
		errorChannel    chan<- RequestError
	}
)

func NewHttpMonitor(amountOfWorkers int, errorChannel chan<- RequestError) *HttpMonitor {
	monitor := new(HttpMonitor)

	monitor.amountOfWorkers = amountOfWorkers
	monitor.requests = make([]EndpointRequest, 0, amountOfWorkers)
	monitor.workerChannel = make(chan EndpointRequest, amountOfWorkers)
	monitor.errorChannel = errorChannel

	return monitor
}

func (m *HttpMonitor) RequestExists(endpoint EndpointRequest) bool {
	index := slices.IndexFunc(m.requests, func(r EndpointRequest) bool {
		return r.ClientId == endpoint.ClientId && r.Endpoint == r.Endpoint
	})
	return index != -1
}

func (m *HttpMonitor) AddRequest(request EndpointRequest) {
	m.requests = append(m.requests, request)
}

func (m *HttpMonitor) StartMonitor() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	for id := 0; id < m.amountOfWorkers; id++ {
		go monitorWorker(ctx, id, m.workerChannel, m.errorChannel)
	}

	for {
		for _, r := range m.requests {
			select {
			case <-ctx.Done():
				close(m.workerChannel)
				return
			default:
			}

			m.workerChannel <- r
		}
	}
}

func (m *HttpMonitor) StopMonitor() {
	m.cancel()
	close(m.workerChannel)
}

func monitorWorker(ctx context.Context, workerId int, workerChannel <-chan EndpointRequest, updateChannel chan<- RequestError) {
	slog.Info("worker is starting", "workerId", workerId)

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping worker", "workerId", workerId)
			return
		case r := <-workerChannel:
			err := checkLiveliness(r.Endpoint)
			updateChannel <- RequestError{
				EndpointRequest: r,
				Error:           err,
			}
		}
	}
}

func checkLiveliness(endpoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(5))
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return errors.New(fmt.Sprintf(
			"Invalid status code. Received: %d, expected: 200 or 201",
			res.StatusCode,
		))
	}

	return nil
}
