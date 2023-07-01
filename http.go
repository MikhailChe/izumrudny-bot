package main

import (
	"net/http"
	"strings"

	. "mikhailche/botcomod/tracer"
)

func TracedHttpClient(botToken string) *http.Client {
	defer Trace("TracedHttpClient")()
	client := http.Client{
		Transport: TracedRoundTripper(botToken),
	}

	return &client
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (t roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return t(r)
}

func TracedRoundTripper(botToken string) roundTripperFunc {
	return func(r *http.Request) (*http.Response, error) {
		url := r.URL.String()
		url = strings.ReplaceAll(url, botToken, "##")
		defer Trace("HTTP::" + url)()
		return http.DefaultTransport.RoundTrip(r)
	}
}
