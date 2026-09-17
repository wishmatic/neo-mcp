package config

import (
	"os"
	"testing"
)

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

func TestLoadImg2TxtValues(t *testing.T) {
	t.Setenv("IMG2TXT_BASE_URL", "https://api.example.com/v1")
	t.Setenv("IMG2TXT_API_KEY", "secret")
	t.Setenv("IMG2TXT_MODEL", "vision")
	t.Setenv("IMG2TXT_SYSTEM_PROMPT", "be terse")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Img2TxtBaseURL != "https://api.example.com/v1" {
		t.Errorf("Img2TxtBaseURL = %q", cfg.Img2TxtBaseURL)
	}

	if cfg.Img2TxtAPIKey != "secret" {
		t.Errorf("Img2TxtAPIKey = %q", cfg.Img2TxtAPIKey)
	}

	if cfg.Img2TxtModel != "vision" {
		t.Errorf("Img2TxtModel = %q", cfg.Img2TxtModel)
	}

	if cfg.Img2TxtSystemPrompt != "be terse" {
		t.Errorf("Img2TxtSystemPrompt = %q", cfg.Img2TxtSystemPrompt)
	}
}
