package http

import (
	"context"
	"mikhailche/botcomod/lib/tracer.v2"
	"net"
	"net/http"
	"strings"
	"time"
)

func TracedHttpClient(ctx context.Context, botToken string) *http.Client {
	ctx, span := tracer.Open(ctx, tracer.Named("TracedHttpClient"))
	defer span.Close()
	client := http.Client{
		Transport: tracedRoundTripper(botToken),
	}

	return &client
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (t roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return t(r)
}

func tracedRoundTripper(botToken string) roundTripperFunc {
	return func(r *http.Request) (*http.Response, error) {
		url := strings.ReplaceAll(r.URL.String(), botToken, "##")
		newctx, span := tracer.Open(r.Context(), tracer.Named("HTTP::"+url))
		defer span.Close()
		return tracedTransport().RoundTrip(r.WithContext(newctx))
	}
}

func tracedTransport() *http.Transport {
	// This is a copy of http.DefaultTransport with a touch of tracing for dialer
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: tracedDialer(defaultTransportDialContext(&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		})),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func tracedDialer(dialContext func(context.Context, string, string) (net.Conn, error)) func(ctx context.Context, network string, addr string) (net.Conn, error) {
	return func(ctx context.Context, network string, addr string) (net.Conn, error) {
		ctx, span := tracer.Open(ctx, tracer.Named("Dial::"+network+"//"+addr))
		defer span.Close()
		return dialContext(ctx, network, addr)
	}
}

func defaultTransportDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return dialer.DialContext
}
