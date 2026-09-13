package config

import (
	"os"
	"testing"
)

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
