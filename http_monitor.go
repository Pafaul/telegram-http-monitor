package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/http"
	"slices"
	"sync"
	"time"
)

type (
	EndpointRequest struct {
		ClientId     int64  `json:"clientId" yaml:"clientId"`
		Endpoint     string `json:"endpoint" yaml:"endpoint"`
		requestError error
	}

	RequestError struct {
		EndpointRequest
		Error error `json:"error"`
	}

	HttpMonitor struct {
		lock            sync.Mutex
		cancel          context.CancelFunc
		requests        []*EndpointRequest
		amountOfWorkers int
		workerChannel   chan *EndpointRequest
		errorChannel    chan<- RequestError
	}
)

func NewHttpMonitor(amountOfWorkers int, errorChannel chan<- RequestError) *HttpMonitor {
	monitor := new(HttpMonitor)

	monitor.amountOfWorkers = amountOfWorkers
	monitor.requests = make([]*EndpointRequest, 0, amountOfWorkers)
	monitor.workerChannel = make(chan *EndpointRequest, amountOfWorkers)
	monitor.errorChannel = errorChannel

	return monitor
}

func (m *HttpMonitor) requestIndex(endpoint EndpointRequest) int {
	index := slices.IndexFunc(m.requests, func(r *EndpointRequest) bool {
		return r.ClientId == endpoint.ClientId && r.Endpoint == endpoint.Endpoint
	})
	return index
}

func (m *HttpMonitor) RequestExists(endpoint EndpointRequest) bool {
	return m.requestIndex(endpoint) != -1
}

func (m *HttpMonitor) AddRequest(request *EndpointRequest) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.requests = append(m.requests, request)
}

func (m *HttpMonitor) RemoveRequest(request EndpointRequest) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	index := m.requestIndex(request)
	if index == -1 {
		return false
	}

	m.requests[index] = m.requests[len(m.requests)-1]
	m.requests = m.requests[:len(m.requests)-1]

	return true
}

func (m *HttpMonitor) ListRequests(client int64) []*EndpointRequest {
	clientRequests := make([]*EndpointRequest, 0, 10)
	for _, r := range m.requests {
		if r.ClientId == client {
			clientRequests = append(clientRequests, r)
		}
	}

	return clientRequests
}

func (m *HttpMonitor) StartMonitor() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	log.Info().Int("amount", m.amountOfWorkers).Msg("starting monitor workers")

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

func monitorWorker(ctx context.Context, workerId int, workerChannel <-chan *EndpointRequest, updateChannel chan<- RequestError) {
	log.Info().Int("workerId", workerId).Msg("worker is starting")

	for {
		select {
		case <-ctx.Done():
			log.Info().Int("workerId", workerId).Msg("stopping worker")
			return
		case r := <-workerChannel:
			err := checkLiveliness(r.Endpoint)
			if err != nil {
				if r.requestError != nil {
					continue
				}
				r.requestError = err
				updateChannel <- RequestError{
					EndpointRequest: *r,
					Error:           err,
				}
				continue
			}

			if r.requestError != nil {
				r.requestError = nil
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
