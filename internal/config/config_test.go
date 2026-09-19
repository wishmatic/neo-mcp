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
	unsetEnv(t, "EXAMPLES_MAX")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.ExamplesEnabled {
		t.Error("ExamplesEnabled = true, want false by default")
	}

	if cfg.ExamplesMax != 16 {
		t.Errorf("ExamplesMax = %d, want 16 by default", cfg.ExamplesMax)
	}
}

func TestLoadExamples(t *testing.T) {
	t.Setenv("EXAMPLES_ENABLED", "true")
	t.Setenv("EXAMPLES_MAX", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !cfg.ExamplesEnabled {
		t.Error("ExamplesEnabled = false, want true")
	}

	if cfg.ExamplesMax != 3 {
		t.Errorf("ExamplesMax = %d, want 3", cfg.ExamplesMax)
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

func TestGaragefrontPrefix(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		want   string
	}{
		{
			name:   "user id set",
			userID: "6a7ee81dea3798015702d047",
			want:   "i/images/6a7ee81dea3798015702d047",
		},
		{
			name: "user id unset",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{GaragefrontUserID: tt.userID}

			if got := cfg.GaragefrontPrefix(); got != tt.want {
				t.Fatalf("GaragefrontPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadS3UsePathStyleDefault(t *testing.T) {
	prev, had := os.LookupEnv("S3_USE_PATH_STYLE")
	os.Unsetenv("S3_USE_PATH_STYLE")

	defer func() {
		if had {
			os.Setenv("S3_USE_PATH_STYLE", prev)

			return
		}

		os.Unsetenv("S3_USE_PATH_STYLE")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !cfg.S3UsePathStyle {
		t.Fatalf("S3UsePathStyle = false, want true when unset")
	}
}

func TestLoadS3UsePathStyleOverride(t *testing.T) {
	t.Setenv("S3_USE_PATH_STYLE", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.S3UsePathStyle {
		t.Fatalf("S3UsePathStyle = true, want false when set")
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
