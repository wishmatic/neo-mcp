package config

import (
	"os"
	"testing"
)

func TestGaragefrontPrefix(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		keyPrefix string
		want      string
	}{
		{
			name:      "user id wins",
			userID:    "6a7ee81dea3798015702d047",
			keyPrefix: "i/mcp",
			want:      "i/images/6a7ee81dea3798015702d047",
		},
		{
			name:      "falls back to key prefix",
			keyPrefix: "i/mcp",
			want:      "i/mcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{GaragefrontUserID: tt.userID, GaragefrontKeyPrefix: tt.keyPrefix}

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
