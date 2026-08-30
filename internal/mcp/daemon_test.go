package mcp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/yougile-mcp/internal/domain/valueobject"
	boardservice "github.com/yougile-mcp/internal/service/board"
)

// newDaemonTestServer поднимает daemon-режим (issue #4) на httptest:
// /mcp — Streamable HTTP MCP, /healthz — health.
func newDaemonTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	s := New(Deps{Daemon: true}) // mode по умолчанию confirm
	mux := http.NewServeMux()
	mux.Handle("/mcp", server.NewStreamableHTTPServer(s.MCPServer()))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	hs := httptest.NewServer(mux)
	t.Cleanup(hs.Close)
	return hs, s
}

// connectAgent подключает клиента с указанным именем (clientInfo handshake).
func connectAgent(t *testing.T, url, name string) (*client.Client, *transport.StreamableHTTP) {
	t.Helper()
	tr, err := transport.NewStreamableHTTP(url)
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	c := client.NewClient(tr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := mcp.InitializeRequest{}
	req.Params.ClientInfo = mcp.Implementation{Name: name, Version: "1.0"}
	req.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	if _, err := c.Initialize(ctx, req); err != nil {
		t.Fatalf("initialize(%s): %v", name, err)
	}
	return c, tr
}

func callMode(t *testing.T, c *client.Client, tool string, args map[string]any) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := mcp.CallToolRequest{}
	req.Params.Name = tool
	req.Params.Arguments = args
	res, err := c.CallTool(ctx, req)
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	var buf bytes.Buffer
	for _, item := range res.Content {
		if tx, ok := item.(mcp.TextContent); ok {
			buf.WriteString(tx.Text)
		}
	}
	return buf.String()
}

// TestDaemonPerSessionModeIsolation — /yougile-mode yolo у агента A
// не меняет режим агента B, глобальный режим сервера не трогается (issue #4).
func TestDaemonPerSessionModeIsolation(t *testing.T) {
	hs, s := newDaemonTestServer(t)

	a, trA := connectAgent(t, hs.URL+"/mcp", "agent-a")
	defer a.Close()
	b, trB := connectAgent(t, hs.URL+"/mcp", "agent-b")
	defer b.Close()

	if got := callMode(t, a, "get_mode", nil); got != `{"mode": "confirm"}` {
		t.Fatalf("agent-a initial mode: %s", got)
	}
	if got := callMode(t, b, "get_mode", nil); got != `{"mode": "confirm"}` {
		t.Fatalf("agent-b initial mode: %s", got)
	}

	if got := callMode(t, a, "set_mode", map[string]any{"mode": "yolo"}); got != `{"mode": "yolo", "ok": true, "scope": "session"}` {
		t.Fatalf("agent-a set_mode: %s", got)
	}

	// A видит yolo, B остался в confirm
	if got := callMode(t, a, "get_mode", nil); got != `{"mode": "yolo"}` {
		t.Fatalf("agent-a after set: %s", got)
	}
	if got := callMode(t, b, "get_mode", nil); got != `{"mode": "confirm"}` {
		t.Fatalf("agent-b must stay confirm: %s", got)
	}

	// Глобальный режим сервера не изменился (конфиг не трогаем)
	if s.Mode() != "confirm" {
		t.Fatalf("server global mode changed: %s", s.Mode())
	}

	// Идентичность из handshake видна в реестре (для sender без явного sender)
	if st, ok := s.sessions.Get(trA.GetSessionId()); !ok || st.Name != "agent-a" {
		t.Fatalf("registry for agent-a: %+v ok=%v", st, ok)
	}
	if st, ok := s.sessions.Get(trB.GetSessionId()); !ok || st.Name != "agent-b" {
		t.Fatalf("registry for agent-b: %+v ok=%v", st, ok)
	}
}

// fakeBoardService — минимальный фейк: нужен только CreateBoard.
type fakeBoardService struct{ boardservice.Service }

func (fakeBoardService) CreateBoard(_ context.Context, _ string, _ valueobject.ProjectID) (valueobject.BoardID, error) {
	id, err := valueobject.NewBoardID("11111111-2222-3333-4444-555555555555")
	if err != nil {
		return valueobject.BoardID{}, err
	}
	return id, nil
}

// TestDaemonReadSessionBlocksMutations — per-session read блокирует мутации
// только для своей сессии: B продолжает работать в confirm.
func TestDaemonReadSessionBlocksMutations(t *testing.T) {
	hs, s := newDaemonTestServer(t)
	s.board = fakeBoardService{}

	a, _ := connectAgent(t, hs.URL+"/mcp", "agent-a")
	defer a.Close()
	b, _ := connectAgent(t, hs.URL+"/mcp", "agent-b")
	defer b.Close()

	callMode(t, a, "set_mode", map[string]any{"mode": "read"})

	// Мутация от A блокируется (read-сессия), от B — проходит до сервиса.
	resA := callMode(t, a, "create_board", map[string]any{"projectId": "<PROJECT_ID>", "title": "x"})
	if !containsSubstr(resA, "read-режиме") {
		t.Fatalf("agent-a mutation should be blocked in read, got: %.200s", resA)
	}
	resB := callMode(t, b, "create_board", map[string]any{"projectId": "<PROJECT_ID>", "title": "x"})
	if containsSubstr(resB, "read-режиме") {
		t.Fatalf("agent-b mutation must not be blocked, got: %.200s", resB)
	}
	_ = s
}

// TestDaemonReconnectKeepsDefaultMode — после переподключения агент получает
// дефолтный режим сервера, а не режим другой сессии (issue #4, п.3 acceptance).
func TestDaemonReconnectKeepsDefaultMode(t *testing.T) {
	hs, _ := newDaemonTestServer(t)

	a, _ := connectAgent(t, hs.URL+"/mcp", "agent-a")
	callMode(t, a, "set_mode", map[string]any{"mode": "yolo"})
	a.Close() // разрыв сессии — registry Forget

	// Новое подключение (другой агент) — дефолтный confirm, не yolo от A
	c, _ := connectAgent(t, hs.URL+"/mcp", "agent-c")
	defer c.Close()
	if got := callMode(t, c, "get_mode", nil); got != `{"mode": "confirm"}` {
		t.Fatalf("reconnected agent must get default confirm, got: %s", got)
	}
}

// TestDaemonHealthEndpoint — /healthz жив.
func TestDaemonHealthEndpoint(t *testing.T) {
	hs, _ := newDaemonTestServer(t)
	resp, err := http.Get(hs.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status: %d", resp.StatusCode)
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
