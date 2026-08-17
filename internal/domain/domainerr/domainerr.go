// Package domainerr — общие доменные ошибки, маппинг HTTP-статусов.
package domainerr

import "errors"

// Доменные ошибки (маппинг HTTP-статусов в infrastructure).
var (
	ErrNotFound     = errors.New("not found")    // 404
	ErrRateLimited  = errors.New("rate limited") // 429
	ErrUnauthorized = errors.New("unauthorized") // 401
	ErrBadRequest   = errors.New("bad request")  // 400
	ErrServerError  = errors.New("server error") // 5xx
	ErrConflict     = errors.New("conflict")     // 409
)
