package main

import (
	"context"
	"database/sql"
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
		amountOfWorkers int
		workerChannel   chan *EndpointRequest
		requestIterator *RequestIterator
	}

	IHttpMonitor interface {
		StartMonitor(ctx context.Context, db *sql.DB, errorChannel chan<- RequestError)
		AddRequest(request *EndpointRequest)
		RemoveRequest(request *EndpointRequest) bool
		RequestExists(request *EndpointRequest) bool
	}
)

func NewHttpMonitor(amountOfWorkers int) *HttpMonitor {
	monitor := new(HttpMonitor)

	monitor.amountOfWorkers = amountOfWorkers
	monitor.workerChannel = make(chan *EndpointRequest, amountOfWorkers)

	monitor.requestIterator = NewRequestIterator(amountOfWorkers)

	return monitor
}

func (m *HttpMonitor) StartMonitor(ctx context.Context, db *sql.DB, errorChannel chan<- RequestError) {
	q := monitor_db.New(db)
	requests, _ := q.GetEndpointsToMonitor(context.Background())
	for _, r := range requests {
		m.requestIterator.Add(&EndpointRequest{
			Endpoint:     r.Url,
			lock:         &sync.Mutex{},
			requestError: nil,
		})
	}

	go m.requestIterator.Start(ctx)

	log.Info().Int("amount", m.amountOfWorkers).Msg("starting monitor workers")

	var wg sync.WaitGroup
	for id := 0; id < m.amountOfWorkers; id++ {
		go func(id int) {
			wg.Add(1)
			monitorWorker(ctx, id, m.workerChannel, errorChannel)
			wg.Done()
		}(id)
	}

	for {
		r, ok := <-m.requestIterator.ReceiveChannel
		if !ok {
			log.Warn().Msg("request iterator has closed the channel")
			return
		}

		select {
		case <-ctx.Done():
			wg.Wait()
			close(m.workerChannel)
			return
		default:
		}

		m.workerChannel <- r
	}
}

func (m *HttpMonitor) AddRequest(request *EndpointRequest) {
	request.lock = &sync.Mutex{}
	m.requestIterator.Add(request)
}

func (m *HttpMonitor) RemoveRequest(request *EndpointRequest) bool {
	return m.requestIterator.Remove(request)
}

func (m *HttpMonitor) RequestExists(request *EndpointRequest) bool {
	return m.requestIterator.RequestExists(request)
}

func monitorWorker(ctx context.Context, workerId int, workerChannel <-chan *EndpointRequest, updateChannel chan<- RequestError) {
	log.Info().Int("workerId", workerId).Msg("worker is starting")

	for {
		select {
		case <-ctx.Done():
			log.Info().Int("workerId", workerId).Msg("stopping worker")
			return
		case r := <-workerChannel:
			log.Info().Int("workerId", workerId).Str("endpoint", r.Endpoint).Msg("requesting")
			r.lock.Lock()
			err := checkLiveliness(http.DefaultClient, r.Endpoint)
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

func checkLiveliness(client *http.Client, endpoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(5))
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	res, err := client.Do(request)
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

var _ IHttpMonitor = (*HttpMonitor)(nil)
