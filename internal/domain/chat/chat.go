// Package chat — чат задачи (taskId == chatId в API YouGile).
package chat

import (
	"context"

	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Message — сообщение в чате задачи.
type Message struct {
	ID         int64  `json:"id"` // timestamp ms — он же ID сообщения
	FromUserID string `json:"fromUserId"`
	Text       string `json:"text"`
}

// Repository — доступ к чату задачи.
type Repository interface {
	// List возвращает страницу сообщений чата задачи.
	List(ctx context.Context, taskID valueobject.TaskID, limit, offset int) ([]Message, valueobject.PagingMetadata, error)
	// Send отправляет текст в чат задачи, возвращает ID сообщения.
	Send(ctx context.Context, taskID valueobject.TaskID, text string) (int64, error)
}
