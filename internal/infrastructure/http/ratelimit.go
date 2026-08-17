package http

import (
	"context"
	"sync"
	"time"
)

// tokenBucket — простой token bucket rate limiter (без внешних зависимостей).
type tokenBucket struct {
	mu       sync.Mutex
	rps      float64 // токенов в секунду
	burst    int     // максимальный размер bucket
	tokens   float64 // текущее количество токенов
	lastFill time.Time
}

// newTokenBucket создаёт bucket с rps токенов/сек и ёмкостью burst.
func newTokenBucket(rps float64, burst int) *tokenBucket {
	return &tokenBucket{
		rps:      rps,
		burst:    burst,
		tokens:   float64(burst),
		lastFill: time.Now(),
	}
}

// wait блокирует до получения токена (или отмены контекста).
func (b *tokenBucket) wait(ctx context.Context) error {
	for {
		b.mu.Lock()
		b.fill()
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}
		// вычислить время до следующего токена
		needed := (1 - b.tokens) / b.rps
		b.mu.Unlock()

		timer := time.NewTimer(time.Duration(needed * float64(time.Second)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// продолжить цикл
		}
	}
}

// fill пополняет bucket по прошедшему времени (вызывается под mu).
func (b *tokenBucket) fill() {
	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * b.rps
	if b.tokens > float64(b.burst) {
		b.tokens = float64(b.burst)
	}
	b.lastFill = now
}
