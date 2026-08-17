// Package repository — HTTP-реализации репозиториев YouGile API v2.
package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/yougile-mcp/internal/domain/domainerr"
)

// client — обёртка над http.Client с base URL и JSON-хелперами.
type client struct {
	http    *http.Client
	baseURL string // например https://ru.yougile.com/api-v2
}

func newClient(hc *http.Client, baseURL string) *client {
	return &client{http: hc, baseURL: baseURL}
}

// get выполняет GET и декодирует JSON-ответ в out.
// Ошибки HTTP-статусов маппятся в доменные ошибки.
func (c *client) get(ctx context.Context, path string, out any) error {
	return c.doJSON(ctx, http.MethodGet, path, nil, out)
}

// post выполняет POST с JSON-телом.
func (c *client) post(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, http.MethodPost, path, body, out)
}

// put выполняет PUT с JSON-телом.
func (c *client) put(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, http.MethodPut, path, body, out)
}

// delete выполняет DELETE.
func (c *client) delete(ctx context.Context, path string) error {
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// doJSON выполняет запрос; body != nil — сериализует в JSON.
func (c *client) doJSON(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("repository: marshal body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("repository: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("repository: do request: %w", err)
	}
	defer resp.Body.Close()

	if err := mapStatus(resp.StatusCode); err != nil {
		return err
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("repository: decode response: %w", err)
	}
	return nil
}

// mapStatus маппит HTTP-статус в доменную ошибку.
func mapStatus(status int) error {
	switch {
	case status == http.StatusNotFound:
		return domainerr.ErrNotFound
	case status == http.StatusTooManyRequests:
		return domainerr.ErrRateLimited
	case status == http.StatusUnauthorized:
		return domainerr.ErrUnauthorized
	case status == http.StatusBadRequest:
		return domainerr.ErrBadRequest
	case status >= 500:
		return domainerr.ErrServerError
	case status >= 400:
		return fmt.Errorf("repository: unexpected status %d", status)
	default:
		return nil
	}
}

// addQuery добавляет query-параметры к path.
func addQuery(path string, params map[string]string) string {
	if len(params) == 0 {
		return path
	}
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return path + "?" + q.Encode()
}

// pagingDTO — структура пагинации в ответе API.
type pagingDTO struct {
	Count  int  `json:"count"`
	Limit  int  `json:"limit"`
	Offset int  `json:"offset"`
	Next   bool `json:"next"`
}
