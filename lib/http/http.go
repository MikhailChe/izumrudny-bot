package http

import (
	"context"
	"mikhailche/botcomod/lib/tracer.v2"
	"net/http"
	"strings"
)

func TracedHttpClient(ctx context.Context, botToken string) *http.Client {
	ctx, span := tracer.Open(ctx, tracer.Named("TracedHttpClient"))
	defer span.Close()
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
		newctx, span := tracer.Open(r.Context(), tracer.Named("HTTP::"+url))
		defer span.Close()
		return http.DefaultTransport.RoundTrip(r.WithContext(newctx))
	}
}
