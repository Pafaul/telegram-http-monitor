package main

import (
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

func (ri *RequestIterator) Next() *EndpointRequest {
	for {
		if len(ri.requests) == 0 {
			continue
		}

		ri.lock.RLock()
		if ri.currentIndex >= len(ri.requests) {
			ri.currentIndex = 0
		}

		ri.ReceiveChannel <- ri.requests[ri.currentIndex]
		ri.lock.RUnlock()

		ri.currentIndex += 1
	}
}

func (ri *RequestIterator) Add(request *EndpointRequest) {
	ri.lock.Lock()
	defer ri.lock.Unlock()
	if ri.requestExists(request) {
		return
	}

	ri.requests = append(ri.requests, request)
}

func (ri *RequestIterator) Remove(request *EndpointRequest) {
	ri.lock.Lock()
	defer ri.lock.Unlock()
	if !ri.requestExists(request) {
		return
	}

	if len(ri.requests) == 1 {
		ri.requests = make([]*EndpointRequest, 0)
	}

	indexToRemove := ri.indexOf(request)
	ri.requests[indexToRemove], ri.requests[len(ri.requests)-1] = ri.requests[len(ri.requests)-1], ri.requests[indexToRemove]
	ri.requests = ri.requests[:len(ri.requests)-1]
}

func (ri *RequestIterator) indexOf(request *EndpointRequest) int {
	index := slices.IndexFunc(ri.requests, func(r *EndpointRequest) bool {
		return request.Endpoint == r.Endpoint && request.ClientId == r.ClientId
	})
	return index
}

func (ri *RequestIterator) requestExists(request *EndpointRequest) bool {
	return ri.indexOf(request) != -1
}
