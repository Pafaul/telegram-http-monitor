package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestErrorConversion(t *testing.T) {
	newErr := new(url.Error)
	otherUrlError := new(url.Error)
	if !errors.As(newErr, &otherUrlError) {
		t.Fatal("Could not catch error")
	}
}

func TestCheckLiveliness(t *testing.T) {
	endpointsToCheck := []EndpointRequest{
		{Endpoint: "https://google.com", RequiredStatus: http.StatusOK, TimeoutInSeconds: 500},
		{Endpoint: "https://facebook.com", RequiredStatus: http.StatusOK, TimeoutInSeconds: 500},
		{Endpoint: "https://yandex.ru", RequiredStatus: http.StatusOK, TimeoutInSeconds: 500},
		{Endpoint: "https://unknown-website.com", RequiredStatus: http.StatusOK, TimeoutInSeconds: 500},
	}

	expectedErrors := []any{
		nil,
		nil,
		nil,
		new(url.Error),
	}

	receivedErrors := CheckEndpoints(endpointsToCheck)

	for i, err := range receivedErrors {
		t.Log(fmt.Sprintf("Checking errors with index: %d", i))
		if expectedErrors[i] == nil && receivedErrors[i] == nil {
			continue
		}
		if !errors.As(err, &expectedErrors[i]) {
			t.Error("Invalid error received")
		}
	}
}
