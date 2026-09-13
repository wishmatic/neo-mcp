package sdwebui

import "testing"

func TestEnsureSafetensors(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"qwen_image_vae", "qwen_image_vae.safetensors"},
		{"animiji_s1_txt.safetensors", "animiji_s1_txt.safetensors"},
		{"model.ckpt", "model.ckpt"},
		{"model.pt", "model.pt"},
		{"model.bin", "model.bin"},
		{"model.gguf", "model.gguf"},
		{"MODEL", "MODEL.safetensors"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := ensureSafetensors(tt.in); got != tt.want {
			t.Errorf("ensureSafetensors(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
