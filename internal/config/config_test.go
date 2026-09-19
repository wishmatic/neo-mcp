package config

import (
	"os"
	"path/filepath"
	"testing"
)

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	prev, had := os.LookupEnv(key)

	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}

	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)

			return
		}

		_ = os.Unsetenv(key)
	})
}

func TestLoadDBPathDefaults(t *testing.T) {
	unsetEnv(t, "DB_PATH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.DBPath != "neo-mcp.db" {
		t.Errorf("DBPath = %q, want neo-mcp.db", cfg.DBPath)
	}
}

func TestLoadDBPath(t *testing.T) {
	t.Setenv("DB_PATH", filepath.Join("data", "neo.db"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.DBPath != filepath.Join("data", "neo.db") {
		t.Errorf("DBPath = %q, want the configured path", cfg.DBPath)
	}
}

func TestLoadExamplesDefaults(t *testing.T) {
	unsetEnv(t, "EXAMPLES_ENABLED")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.ExamplesEnabled {
		t.Error("ExamplesEnabled = true, want false by default")
	}
}

func TestLoadExamples(t *testing.T) {
	t.Setenv("EXAMPLES_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !cfg.ExamplesEnabled {
		t.Error("ExamplesEnabled = false, want true")
	}
}

func TestLoadOutputFormatDefaults(t *testing.T) {
	unsetEnv(t, "OUTPUT_FORMAT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.OutputFormat != "webp" {
		t.Errorf("OutputFormat = %q, want webp", cfg.OutputFormat)
	}
}

func TestLoadOutputFormat(t *testing.T) {
	tests := map[string]string{
		"jxl":  "jxl",
		"jpeg": "jpeg",
		"JPEG": "JPEG",
	}

	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			t.Setenv("OUTPUT_FORMAT", value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			if cfg.OutputFormat != want {
				t.Errorf("OutputFormat = %q, want %q", cfg.OutputFormat, want)
			}
		})
	}
}

func TestLoadFilesDefaults(t *testing.T) {
	unsetEnv(t, "PUBLIC_HOST")
	unsetEnv(t, "FILES_DIR")
	unsetEnv(t, "FILES_RETENTION_DAYS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.PublicHost != "" {
		t.Errorf("PublicHost = %q, want empty by default", cfg.PublicHost)
	}

	if cfg.FilesDir != "files" {
		t.Errorf("FilesDir = %q, want files by default", cfg.FilesDir)
	}

	if cfg.FilesRetentionDays != 0 {
		t.Errorf("FilesRetentionDays = %d, want 0 by default", cfg.FilesRetentionDays)
	}
}

func TestLoadFiles(t *testing.T) {
	t.Setenv("PUBLIC_HOST", "https://neo.example.com")
	t.Setenv("FILES_DIR", filepath.Join("data", "files"))
	t.Setenv("FILES_RETENTION_DAYS", "30")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.PublicHost != "https://neo.example.com" {
		t.Errorf("PublicHost = %q, want https://neo.example.com", cfg.PublicHost)
	}

	if cfg.FilesDir != filepath.Join("data", "files") {
		t.Errorf("FilesDir = %q, want the configured path", cfg.FilesDir)
	}

	if cfg.FilesRetentionDays != 30 {
		t.Errorf("FilesRetentionDays = %d, want 30", cfg.FilesRetentionDays)
	}
}

func TestPublicBase(t *testing.T) {
	tests := []struct {
		name       string
		publicHost string
		want       string
		wantErr    bool
	}{
		{name: "unset", publicHost: "", wantErr: false},
		{name: "valid", publicHost: "https://neo.example.com", want: "https://neo.example.com"},
		{name: "valid with port", publicHost: "http://192.168.1.10:8080", want: "http://192.168.1.10:8080"},
		{name: "trailing slash trimmed", publicHost: "https://neo.example.com/", want: "https://neo.example.com"},
		{name: "missing scheme", publicHost: "neo.example.com", wantErr: true},
		{name: "non-http scheme", publicHost: "ftp://neo.example.com", wantErr: true},
		{name: "empty host", publicHost: "https://", wantErr: true},
		{name: "path component", publicHost: "https://neo.example.com/neo", wantErr: true},
		{name: "query component", publicHost: "https://neo.example.com?x=1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{PublicHost: tt.publicHost}

			base, err := cfg.PublicBase()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PublicBase() error = nil, want an error")
				}

				return
			}

			if err != nil {
				t.Fatalf("PublicBase() error: %v", err)
			}

			if tt.publicHost == "" {
				if base != nil {
					t.Fatalf("PublicBase() = %v, want nil when PUBLIC_HOST is unset", base)
				}

				return
			}

			if base == nil || base.Host == "" {
				t.Fatalf("PublicBase() = %v, want a URL with a host", base)
			}

			if got := base.String(); got != tt.want {
				t.Errorf("PublicBase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadNovelAIAPIKey(t *testing.T) {
	t.Setenv("NOVELAI_API_KEY", "sk-test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.NovelAIAPIKey != "sk-test" {
		t.Errorf("NovelAIAPIKey = %q, want sk-test", cfg.NovelAIAPIKey)
	}
}
