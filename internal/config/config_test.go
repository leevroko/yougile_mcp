package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeConfig пишет конфиг с заданными правами файла и каталога.
func writeConfig(t *testing.T, dirMode, fileMode os.FileMode, cfg Config) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, fileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, fileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, dirMode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadOK(t *testing.T) {
	path := writeConfig(t, 0o700, 0o600, Config{APIKey: "k"})
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "k" {
		t.Fatalf("api key mismatch")
	}
	// defaults
	if cfg.BaseURL != "https://ru.yougile.com/api-v2" {
		t.Errorf("base url default: %s", cfg.BaseURL)
	}
	if len(cfg.Permissions.Allow) == 0 || len(cfg.Permissions.Confirm) == 0 {
		t.Error("default permissions must be applied")
	}
}

func TestLoadRejectsInsecureFilePerms(t *testing.T) {
	path := writeConfig(t, 0o700, 0o644, Config{APIKey: "k"})
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for file 0644")
	}
}

func TestLoadRejectsInsecureDirPerms(t *testing.T) {
	path := writeConfig(t, 0o755, 0o600, Config{APIKey: "k"})
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for dir 0755")
	}
}

func TestAllowInsecureOverride(t *testing.T) {
	path := writeConfig(t, 0o755, 0o644, Config{APIKey: "k", AllowInsecure: true})
	if _, err := Load(path); err != nil {
		t.Fatalf("allow_insecure must bypass: %v", err)
	}
}

func TestInitWritesSecureConfig(t *testing.T) {
	// Init пишет в дефолтный путь (~/.config/...) — для теста перезапишем
	// реализацию через переменную, но проще проверить через временный HOME.
	t.Setenv("HOME", t.TempDir())
	// Note: os.UserHomeDir кэширует? Нет, читает $HOME каждый раз в Go.
	path, err := Init("test-key")
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file perms %o, want 600", fi.Mode().Perm())
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("dir perms %o, want 700", di.Mode().Perm())
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "test-key" {
		t.Fatal("api key not persisted")
	}
}
