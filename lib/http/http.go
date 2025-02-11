package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mikhailche/botcomod/lib/tracer.v2"
	"net"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

var sensitiveHeaders = []string{
	"Authorization",
	"Proxy-Authorization",
	"Set-Cookie",
	"Cookie",
	"X-Api-Key",
	"X-Amz-Security-Token",
	"X-Amz-Date",
	"X-Amz-Content-Sha256",
	"X-Amz-Target",
	"X-Amz-User-Agent",
	"X-Amz-Request-Id",
	"X-Amz-Id-2",
	"X-Amz-Access-Token",
	"X-Amz-Secret-Access-Key",
	"X-Amz-Signature",
	"X-Amz-Security-Token",
	"X-Amz-Expires",
	"X-Amz-Algorithm",
	"X-Amz-Credential",
	"X-Amz-Date",
	"X-Amz-SignedHeaders",
}

func TracedHttpClient(ctx context.Context, logger *zap.Logger, secrets ...string) *http.Client {
	ctx, span := tracer.Open(ctx, tracer.Named("TracedHttpClient"))
	defer span.Close()
	client := http.Client{
		Transport: tracedRoundTripper(logger, secrets...),
	}

	return &client
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (t roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return t(r)
}

func tracedRoundTripper(logger *zap.Logger, secrets ...string) roundTripperFunc {
	return func(r *http.Request) (*http.Response, error) {
		callID := generateCallID()
		url := maskSecretsInString(r.URL.String(), secrets...)
		newctx, span := tracer.Open(r.Context(), tracer.Named("HTTP::"+url))
		defer span.Close()

		// Log request
		logRequest(r, logger, callID, secrets...)

		resp, err := tracedTransport().RoundTrip(r.WithContext(newctx))
		if err != nil {
			logError(r, err, logger, callID, secrets...)
			return nil, err
		}

		// Log response
		logResponse(resp, logger, callID, secrets...)

		return resp, nil
	}
}

func logRequest(r *http.Request, logger *zap.Logger, callID string, secrets ...string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to read request body", zap.Error(err))
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset body for further use

	// Mask secrets in URL, headers, and body
	maskedURL := maskSecretsInString(r.URL.String(), secrets...)
	maskedHeaders := maskSecrets(r.Header, secrets...)
	maskedBody := maskSecretsInString(string(body), secrets...)

	var jsonBody interface{}
	if json.Unmarshal([]byte(maskedBody), &jsonBody) == nil {
		logger.Info("HTTP Request",
			zap.String("call_id", callID),
			zap.String("url", maskedURL),
			zap.Any("headers", maskedHeaders),
			zap.Any("body", jsonBody),
		)
	} else {
		logger.Info("HTTP Request",
			zap.String("call_id", callID),
			zap.String("url", maskedURL),
			zap.Any("headers", maskedHeaders),
			zap.String("body", maskedBody),
		)
	}
}

func logResponse(resp *http.Response, logger *zap.Logger, callID string, secrets ...string) {
	body, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset body for further use

	// Mask secrets in headers and body
	maskedHeaders := maskSecrets(resp.Header, secrets...)
	maskedBody := maskSecretsInString(string(body), secrets...)

	var jsonBody interface{}
	if json.Unmarshal([]byte(maskedBody), &jsonBody) == nil {
		logger.Info("HTTP Response",
			zap.String("call_id", callID),
			zap.Int("status_code", resp.StatusCode),
			zap.Any("headers", maskedHeaders),
			zap.Any("body", jsonBody),
		)
	} else {
		logger.Info("HTTP Response",
			zap.String("call_id", callID),
			zap.Int("status_code", resp.StatusCode),
			zap.Any("headers", maskedHeaders),
			zap.String("body", maskedBody),
		)
	}
}

func logError(r *http.Request, err error, logger *zap.Logger, callID string, secrets ...string) {
	body, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		logger.Error("Failed to read request body", zap.Error(readErr))
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset body for further use

	// Mask secrets in URL, headers, and body
	maskedURL := maskSecretsInString(r.URL.String(), secrets...)
	maskedHeaders := maskSecrets(r.Header, secrets...)
	maskedBody := maskSecretsInString(string(body), secrets...)

	var jsonBody interface{}
	if json.Unmarshal([]byte(maskedBody), &jsonBody) == nil {
		logger.Error("HTTP Error",
			zap.String("call_id", callID),
			zap.String("url", maskedURL),
			zap.Any("headers", maskedHeaders),
			zap.Any("body", jsonBody),
			zap.Error(err),
		)
	} else {
		logger.Error("HTTP Error",
			zap.String("call_id", callID),
			zap.String("url", maskedURL),
			zap.Any("headers", maskedHeaders),
			zap.String("body", maskedBody),
			zap.Error(err),
		)
	}
}

func maskSecrets(headers http.Header, secrets ...string) http.Header {
	maskedHeaders := headers.Clone()
	for key, values := range maskedHeaders {
		for i, value := range values {
			if contains(sensitiveHeaders, key) {
				maskedHeaders[key][i] = "##"
			} else {
				maskedHeaders[key][i] = maskSecretsInString(value, secrets...)
			}
		}
	}
	return maskedHeaders
}

func maskSecretsInString(s string, secrets ...string) string {
	masked := s
	for _, secret := range secrets {
		masked = strings.ReplaceAll(masked, secret, "##")
	}
	return masked
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func generateCallID() string {
	return fmt.Sprintf("%d", rand.Int())
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
