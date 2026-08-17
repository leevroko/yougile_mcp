// Package audit — аудит мутаций: append-only JSONL-лог.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry — запись аудита.
type Entry struct {
	TS      string         `json:"ts"`
	Tool    string         `json:"tool"`
	Outcome string         `json:"outcome"` // ok | error | read-only-blocked
	Args    map[string]any `json:"args"`
}

// Logger пишет записи аудита мутаций.
type Logger interface {
	Log(tool string, outcome string, args map[string]any)
}

// NewFileLogger создаёт логгер, пишущий JSONL в path.
func NewFileLogger(path string) Logger {
	return &fileLogger{path: path}
}

// NewNoopLogger — заглушка (аудит выключен).
func NewNoopLogger() Logger { return noopLogger{} }

type fileLogger struct {
	path string
	mu   sync.Mutex
}

// Log добавляет запись в JSONL-файл (создаёт каталог 700).
func (l *fileLogger) Log(tool, outcome string, args map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := Entry{
		TS:      time.Now().UTC().Format(time.RFC3339),
		Tool:    tool,
		Outcome: outcome,
		Args:    args,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}

	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, string(line))
}

type noopLogger struct{}

func (noopLogger) Log(string, string, map[string]any) {}
