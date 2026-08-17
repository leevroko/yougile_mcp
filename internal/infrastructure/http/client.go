// Package http — HTTP-клиент YouGile API v2 с RoundTripper-цепочкой:
// BearerAuth → RateLimiter → Retry → base transport.
package http

import (
	"fmt"
	"net/http"
	"time"
)

// Config — конфигурация HTTP-клиента.
type Config struct {
	BaseURL    string        // например https://ru.yougile.com/api-v2
	APIKey     string        // Bearer-токен
	RateLimit  float64       // запросов в секунду (50 req/min = 0.8333)
	Burst      int           // размер bucket (по умолчанию 1)
	MaxRetries int           // количество повторов при 429/5xx
	Timeout    time.Duration // таймаут запроса (по умолчанию 30s)
}

// DefaultRateLimit — 50 req/min по API YouGile.
const DefaultRateLimit = 50.0 / 60.0

// NewClient создаёт http.Client с настроенной RoundTripper-цепочкой.
func NewClient(cfg Config) (*http.Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("http: base URL is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("http: API key is required")
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = DefaultRateLimit
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 1
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	var rt http.RoundTripper = http.DefaultTransport
	rt = &retryTransport{rt: rt, maxRetries: cfg.MaxRetries}
	rt = newRateLimitTransport(rt, cfg.RateLimit, cfg.Burst)
	rt = &bearerAuthTransport{rt: rt, token: cfg.APIKey}

	return &http.Client{
		Transport: rt,
		Timeout:   cfg.Timeout,
	}, nil
}

// bearerAuthTransport добавляет заголовок Authorization: Bearer {token}.
type bearerAuthTransport struct {
	rt    http.RoundTripper
	token string
}

func (t *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.rt.RoundTrip(req)
}

// rateLimitTransport — token bucket rate limiter.
// Блокирует (через context) если bucket пуст.
type rateLimitTransport struct {
	rt     http.RoundTripper
	bucket *tokenBucket
}

func newRateLimitTransport(rt http.RoundTripper, rps float64, burst int) *rateLimitTransport {
	return &rateLimitTransport{rt: rt, bucket: newTokenBucket(rps, burst)}
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.bucket.wait(req.Context()); err != nil {
		return nil, err
	}
	return t.rt.RoundTrip(req)
}

// retryTransport — повторы при 429 и 5xx с exponential backoff.
type retryTransport struct {
	rt         http.RoundTripper
	maxRetries int
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoffDuration(attempt)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
		}
		resp, err = t.rt.RoundTrip(req)
		if err != nil {
			continue // сетевая ошибка — повторить
		}
		if !isRetryable(resp.StatusCode) {
			return resp, nil
		}
		resp.Body.Close() // освободить соединение перед повтором
	}

	return resp, err
}

// isRetryable возвращает true для 429 и 5xx.
func isRetryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// backoffDuration — exponential backoff: 500ms, 1s, 2s, 4s, ...
func backoffDuration(attempt int) time.Duration {
	ms := 500 << (attempt - 1) // 500ms * 2^(attempt-1)
	if ms > 8000 {
		ms = 8000 // кап 8s
	}
	return time.Duration(ms) * time.Millisecond
}
