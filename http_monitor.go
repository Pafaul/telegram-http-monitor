package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/http"
	"pafaul/telegram-http-monitor/monitor_db"
	"sync"
	"time"
)

type (
	EndpointRequest struct {
		ClientId     int64  `json:"clientId" yaml:"clientId"`
		Endpoint     string `json:"endpoint" yaml:"endpoint"`
		lock         sync.Locker
		requestError error
	}

	RequestError struct {
		EndpointRequest
		Error error `json:"error"`
	}

	HttpMonitor struct {
		lock            sync.Mutex
		cancel          context.CancelFunc
		amountOfWorkers int
		workerChannel   chan *EndpointRequest
		errorChannel    chan<- RequestError
	}
)

var (
	requestIterator *RequestIterator
)

func NewHttpMonitor(amountOfWorkers int, errorChannel chan<- RequestError) *HttpMonitor {
	monitor := new(HttpMonitor)

	monitor.amountOfWorkers = amountOfWorkers
	monitor.workerChannel = make(chan *EndpointRequest, amountOfWorkers)
	monitor.errorChannel = errorChannel

	requestIterator = NewRequestIterator(amountOfWorkers)

	return monitor
}

func (m *HttpMonitor) StartMonitor(q *monitor_db.Queries) {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	requests, _ := q.GetAllRequests(context.Background())
	for _, r := range requests {
		requestIterator.Add(&EndpointRequest{
			ClientId:     r.Clientid,
			Endpoint:     r.Endpoint,
			lock:         &sync.Mutex{},
			requestError: nil,
		})
	}

	go requestIterator.Start(ctx)

	log.Info().Int("amount", m.amountOfWorkers).Msg("starting monitor workers")

	for id := 0; id < m.amountOfWorkers; id++ {
		go monitorWorker(ctx, id, m.workerChannel, m.errorChannel)
	}

	for {
		r, ok := <-requestIterator.ReceiveChannel
		if !ok {
			log.Warn().Msg("request iterator has closed the channel")
			return
		}

		select {
		case <-ctx.Done():
			close(m.workerChannel)
			return
		default:
		}

		m.workerChannel <- r
	}
}

func (m *HttpMonitor) AddRequest(request *EndpointRequest) {
	request.lock = &sync.Mutex{}
	requestIterator.Add(request)
}

func (m *HttpMonitor) RemoveRequest(request *EndpointRequest) bool {
	return requestIterator.Remove(request)
}

func (m *HttpMonitor) RequestExists(request *EndpointRequest) bool {
	return requestIterator.RequestExists(request)
}

func (m *HttpMonitor) StopMonitor() {
	m.cancel()
}

func monitorWorker(ctx context.Context, workerId int, workerChannel <-chan *EndpointRequest, updateChannel chan<- RequestError) {
	log.Info().Int("workerId", workerId).Msg("worker is starting")

	for {
		select {
		case <-ctx.Done():
			log.Info().Int("workerId", workerId).Msg("stopping worker")
			return
		case r := <-workerChannel:
			log.Info().Int("workderId", workerId).Str("endpoint", r.Endpoint).Msg("requesting")
			r.lock.Lock()
			err := checkLiveliness(r.Endpoint)
			if err != nil && r.requestError == nil {
				r.requestError = err
				updateChannel <- RequestError{
					EndpointRequest: *r,
					Error:           err,
				}
			}

			if r.requestError != nil && err == nil {
				r.requestError = nil
			}
			r.lock.Unlock()
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
