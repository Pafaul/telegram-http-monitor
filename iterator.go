package main

import (
	"context"
	"github.com/rs/zerolog/log"
	"slices"
	"sync"
)

type (
	RequestIterator struct {
		ReceiveChannel chan *EndpointRequest
		lock           sync.RWMutex
		requests       []*EndpointRequest
		currentIndex   int
	}
)

func NewRequestIterator(numberOfElements int) *RequestIterator {
	ri := new(RequestIterator)

	ri.ReceiveChannel = make(chan *EndpointRequest)
	ri.lock = sync.RWMutex{}
	ri.requests = make([]*EndpointRequest, 0, numberOfElements)
	ri.currentIndex = 0

	return ri
}

// Start
// Iterator must be started as a goroutine so the iterator
// can send values over a blocking channel.
//
// In this case iterator will function as a classic iterator,
// next value will not be handled over the channel until
// the previous one is read
func (ri *RequestIterator) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ri.lock.RLock()
		if len(ri.requests) == 0 {
			continue
		}

		if ri.currentIndex >= len(ri.requests) {
			ri.currentIndex = 0
		}

		log.Info().Str("endpoint", ri.requests[ri.currentIndex].Endpoint).Msg("adding to receive channel")
		ri.ReceiveChannel <- ri.requests[ri.currentIndex]
		ri.lock.RUnlock()

		ri.currentIndex += 1
	}
}

func (ri *RequestIterator) Add(request *EndpointRequest) {
	ri.lock.Lock()
	defer ri.lock.Unlock()
	if ri.RequestExists(request) {
		return
	}

	ri.requests = append(ri.requests, request)
}

func (ri *RequestIterator) Remove(request *EndpointRequest) bool {
	ri.lock.Lock()
	defer ri.lock.Unlock()
	if !ri.RequestExists(request) {
		return false
	}

	if len(ri.requests) == 1 {
		ri.requests = make([]*EndpointRequest, 0)
	}

	indexToRemove := ri.indexOf(request)
	ri.requests[indexToRemove], ri.requests[len(ri.requests)-1] = ri.requests[len(ri.requests)-1], ri.requests[indexToRemove]
	ri.requests = ri.requests[:len(ri.requests)-1]

	return true
}

func (ri *RequestIterator) indexOf(request *EndpointRequest) int {
	index := slices.IndexFunc(ri.requests, func(r *EndpointRequest) bool {
		return request.Endpoint == r.Endpoint && request.ClientId == r.ClientId
	})
	return index
}

func (ri *RequestIterator) RequestExists(request *EndpointRequest) bool {
	return ri.indexOf(request) != -1
}
