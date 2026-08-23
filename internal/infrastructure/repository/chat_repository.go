package repository

import (
	"context"
	"net/http"
	"strconv"

	"github.com/yougile-mcp/internal/domain/chat"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// chatRepository — HTTP-реализация chat.Repository (/chats/{taskId}/messages).
type chatRepository struct {
	client *client
}

// NewChatRepository создаёт chat.Repository.
func NewChatRepository(hc *http.Client, baseURL string) chat.Repository {
	return &chatRepository{client: newClient(hc, baseURL)}
}

// messageDTO — сообщение чата.
type messageDTO struct {
	ID         int64  `json:"id"`
	FromUserID string `json:"fromUserId"`
	Text       string `json:"text"`
}

// chatMessagesListDTO — ответ GET /chats/{id}/messages.
type chatMessagesListDTO struct {
	Paging  pagingDTO    `json:"paging"`
	Content []messageDTO `json:"content"`
}

// sendChatMessageDTO — тело POST /chats/{id}/messages.
type sendChatMessageDTO struct {
	Text string `json:"text"`
}

func (r *chatRepository) List(ctx context.Context, taskID valueobject.TaskID, limit, offset int) ([]chat.Message, valueobject.PagingMetadata, error) {
	var dto chatMessagesListDTO
	if err := r.client.get(ctx, "/chats/"+taskID.String()+"/messages?limit="+strconv.Itoa(limit)+"&offset="+strconv.Itoa(offset), &dto); err != nil {
		return nil, valueobject.PagingMetadata{}, err
	}
	out := make([]chat.Message, 0, len(dto.Content))
	for _, m := range dto.Content {
		out = append(out, chat.Message{ID: m.ID, FromUserID: m.FromUserID, Text: m.Text})
	}
	return out, valueobject.PagingMetadata{
		Count: dto.Paging.Count, Limit: dto.Paging.Limit,
		Offset: dto.Paging.Offset, Next: dto.Paging.Next,
	}, nil
}

func (r *chatRepository) Send(ctx context.Context, taskID valueobject.TaskID, text string) (int64, error) {
	var out struct {
		ID int64 `json:"id"`
	}
	if err := r.client.post(ctx, "/chats/"+taskID.String()+"/messages", sendChatMessageDTO{Text: text}, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}
