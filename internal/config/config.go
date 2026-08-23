// Package config — загрузка конфигурации yougile-mcp.
// Credentials хранятся в ~/.config/yougile-mcp/config.json (chmod 600),
// вне переменных окружения.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Mode — режим доступа к инструментам.
type Mode string

// Режимы доступа.
const (
	// ModeRead — только чтение; мутационные инструменты недоступны.
	ModeRead Mode = "read"
	// ModeConfirm — запись требует подтверждения пользователя (в расширении pi).
	ModeConfirm Mode = "confirm"
	// ModeYolo — все запросы разрешены без подтверждения.
	ModeYolo Mode = "yolo"
)

// ValidMode проверяет корректность режима.
func ValidMode(m string) bool {
	switch Mode(m) {
	case ModeRead, ModeConfirm, ModeYolo:
		return true
	default:
		return false
	}
}

// Permissions — политика инструментов для pi-расширения (glob-паттерны).
type Permissions struct {
	Allow   []string `json:"allow"`
	Confirm []string `json:"confirm"`
	Deny    []string `json:"deny"`
}

// Audit — настройки аудита мутаций.
type Audit struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

// Config — полная конфигурация сервера.
type Config struct {
	APIKey          string      `json:"api_key"`
	BaseURL         string      `json:"base_url"`
	MemoryDir       string      `json:"memory_dir"`
	Mode            Mode        `json:"mode"`      // read | confirm | yolo (default confirm)
	ReadOnly        bool        `json:"read_only"` // legacy: true = ModeRead
	AllowInsecure   bool        `json:"allow_insecure"`
	AgentID         string      `json:"agent_id"` // идентификатор агента для префиксов сообщений в чатах задач
	BulkDryRunFirst *bool       `json:"bulk_dry_run_first,omitempty"`
	Permissions     Permissions `json:"permissions"`
	Audit           Audit       `json:"audit"`
}

// DefaultPath возвращает путь конфига по умолчанию: ~/.config/yougile-mcp/config.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: home dir: %w", err)
	}
	return filepath.Join(home, ".config", "yougile-mcp", "config.json"), nil
}

// ResolvePath определяет путь к конфигу: явный путь > YOUGILE_CONFIG (только путь!) > дефолт.
func ResolvePath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if env := os.Getenv("YOUGILE_CONFIG"); env != "" {
		return env, nil
	}
	return DefaultPath()
}

// Load читает и валидирует конфиг. Отказывается работать при
// неправильных правах (файл ≠ 600, каталог ≠ 700), если не allow_insecure.
func Load(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if err := checkPermissions(path, cfg.AllowInsecure); err != nil {
		return cfg, err
	}

	// Идентификатор агента: env (харнесс выставляет процессу) приоритетнее файла.
	if env := os.Getenv("YOUGILE_AGENT_ID"); env != "" {
		cfg.AgentID = env
	}

	cfg.applyDefaults(path)
	return cfg, nil
}

// checkPermissions проверяет права файла (600) и каталога (700).
func checkPermissions(path string, allowInsecure bool) error {
	if allowInsecure {
		return nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("config: stat: %w", err)
	}
	if fi.Mode().Perm() != 0o600 {
		return fmt.Errorf("config: insecure file permissions %o on %s: require 600 (или установите allow_insecure)", fi.Mode().Perm(), path)
	}
	dir := filepath.Dir(path)
	di, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("config: stat dir: %w", err)
	}
	if di.Mode().Perm() != 0o700 {
		return fmt.Errorf("config: insecure dir permissions %o on %s: require 700 (или установите allow_insecure)", di.Mode().Perm(), dir)
	}
	return nil
}

// applyDefaults заполняет незаданные значения.
func (c *Config) applyDefaults(path string) {
	// Режим: legacy read_only=true → read; иначе confirm (по умолчанию безопасный).
	if c.ReadOnly {
		c.Mode = ModeRead
	} else if c.Mode == "" {
		c.Mode = ModeConfirm
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://ru.yougile.com/api-v2"
	}
	if c.MemoryDir == "" {
		c.MemoryDir = homePath(".local/share/yougile-mcp/memory/reviews")
	}
	if !hasGlob(c.Permissions.Allow) && !hasGlob(c.Permissions.Confirm) && !hasGlob(c.Permissions.Deny) {
		c.Permissions = defaultPermissions()
	}
	if c.Audit.Path == "" {
		c.Audit.Path = homePath(".local/state/yougile-mcp/audit.jsonl")
	}
	_ = path
}

// DefaultPermissions — дефолтная политика: чтение allow, мутации confirm.
func defaultPermissions() Permissions {
	return Permissions{
		Allow: []string{
			"list_*", "get_*", "get_board_snapshot", "summarize_board",
			"track_goals", "compress_reviews",
		},
		Confirm: []string{
			"create_task", "update_task", "audit_board",
			"bulk_move_tasks", "batch_update_stickers",
		},
		Deny: []string{},
	}
}

// SaveMode сохраняет новый режим в конфиг-файл (права сохраняются 600).
func (c *Config) SaveMode(m Mode, path string) error {
	c.Mode = m
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("config: chmod: %w", err)
	}
	return nil
}

// Init создаёт конфиг с ключом из env (одноразовая миграция) с правами 600/700.
// Возвращает созданный путь.
func Init(apiKey string) (string, error) {
	path, err := DefaultPath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("config init: mkdir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("config init: chmod dir: %w", err)
	}

	cfg := Config{
		APIKey:      apiKey,
		Mode:        ModeConfirm,
		Permissions: defaultPermissions(),
		Audit:       Audit{Enabled: true},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("config init: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("config init: write: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("config init: chmod: %w", err)
	}
	return path, nil
}

// hasGlob — непустой ли список паттернов.
func hasGlob(pats []string) bool { return len(pats) > 0 }

// homePath разворачивает относительный к дому путь.
func homePath(rel string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return rel
	}
	return filepath.Join(home, filepath.FromSlash(rel))
}
