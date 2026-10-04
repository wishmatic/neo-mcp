package openai

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestGenerateImagesPostsTheRequest(t *testing.T) {
	server, captured := newImageServer(t, http.StatusOK, `{"data":[{"b64_json":"cG5nLWJ5dGVz"}]}`)

	client := New([]Provider{testProvider(t, server.URL)})

	n := 2
	seed := 42
	steps := 28
	guidance := 3.5
	strength := 0.6

	_, err := client.GenerateImages(context.Background(), "", ImageRequest{
		Model:             "flux-schnell",
		Prompt:            "a cat",
		N:                 &n,
		Size:              "1024x1024",
		ImageDataURL:      "data:image/png;base64,cG5n",
		ImageDataURLs:     []string{"data:image/png;base64,cG5n"},
		MaskDataURL:       "data:image/png;base64,bWFzaw==",
		Seed:              &seed,
		Strength:          &strength,
		GuidanceScale:     &guidance,
		NumInferenceSteps: &steps,
	})
	if err != nil {
		t.Fatalf("GenerateImages() error: %v", err)
	}

	if captured.path != imagesPath {
		t.Errorf("path = %q, want %s", captured.path, imagesPath)
	}

	if captured.authorization != "Bearer sk-test" {
		t.Errorf("Authorization = %q, want the configured key", captured.authorization)
	}

	if captured.contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", captured.contentType)
	}

	body := bodyOf(t, captured)

	for field, want := range map[string]any{
		"model":               "flux-schnell",
		"prompt":              "a cat",
		"n":                   float64(2),
		"size":                "1024x1024",
		"response_format":     ResponseFormatBase64,
		"imageDataUrl":        "data:image/png;base64,cG5n",
		"maskDataUrl":         "data:image/png;base64,bWFzaw==",
		"seed":                float64(42),
		"strength":            0.6,
		"guidance_scale":      3.5,
		"num_inference_steps": float64(28),
	} {
		if got := body[field]; got != want {
			t.Errorf("%s = %v, want %v", field, got, want)
		}
	}

	images, ok := body["imageDataUrls"].([]any)
	if !ok || len(images) != 1 || images[0] != "data:image/png;base64,cG5n" {
		t.Errorf("imageDataUrls = %v, want the one inline input", body["imageDataUrls"])
	}
}

func TestGenerateImagesOmitsUnsetFields(t *testing.T) {
	server, captured := newImageServer(t, http.StatusOK, `{"data":[{"b64_json":"cG5n"}]}`)

	client := New([]Provider{testProvider(t, server.URL)})

	if _, err := client.GenerateImages(context.Background(), "", ImageRequest{Prompt: "a cat"}); err != nil {
		t.Fatalf("GenerateImages() error: %v", err)
	}

	for _, field := range []string{"model", "n", "size", "seed", "strength", "guidance_scale", "num_inference_steps"} {
		if _, present := bodyOf(t, captured)[field]; present {
			t.Errorf("%s is present, want it dropped when the caller left it out", field)
		}
	}
}

func TestGenerateImagesKeepsANamedResponseFormat(t *testing.T) {
	server, captured := newImageServer(t, http.StatusOK, `{"data":[{"b64_json":"cG5n"}]}`)

	client := New([]Provider{testProvider(t, server.URL)})

	_, err := client.GenerateImages(context.Background(), "", ImageRequest{
		Prompt:         "a cat",
		ResponseFormat: "url",
	})
	if err != nil {
		t.Fatalf("GenerateImages() error: %v", err)
	}

	if got := bodyOf(t, captured)["response_format"]; got != "url" {
		t.Errorf("response_format = %v, want the named url", got)
	}
}

func TestGenerateImagesSendsNoAuthorizationWithoutAKey(t *testing.T) {
	server, captured := newImageServer(t, http.StatusOK, `{"data":[{"b64_json":"cG5n"}]}`)

	client := New([]Provider{{Name: "local", BaseURL: server.URL}})

	if _, err := client.GenerateImages(context.Background(), "", ImageRequest{Prompt: "a cat"}); err != nil {
		t.Fatalf("GenerateImages() error: %v", err)
	}

	if captured.authorization != "" {
		t.Errorf("Authorization = %q, want none without a key", captured.authorization)
	}
}

func TestGenerateImagesDecodesInlineBytes(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
	}{
		{name: "standard", encoded: base64.StdEncoding.EncodeToString([]byte("png-bytes"))},
		{name: "unpadded", encoded: base64.RawStdEncoding.EncodeToString([]byte("png-bytes"))},
		{name: "data URI", encoded: "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png-bytes"))},
		{name: "padded with whitespace", encoded: " " + base64.StdEncoding.EncodeToString([]byte("png-bytes")) + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newImageServer(t, http.StatusOK, `{"data":[{"b64_json":`+jsonString(tt.encoded)+`}]}`)

			client := New([]Provider{testProvider(t, server.URL)})

			result, err := client.GenerateImages(context.Background(), "", ImageRequest{Prompt: "a cat"})
			if err != nil {
				t.Fatalf("GenerateImages() error: %v", err)
			}

			if len(result.Images) != 1 {
				t.Fatalf("images = %d, want 1", len(result.Images))
			}

			if got := result.Images[0].Data; string(got) != "png-bytes" {
				t.Errorf("images[0] = %q, want png-bytes", got)
			}
		})
	}
}

func TestGenerateImagesKeepsAURLAnswer(t *testing.T) {
	server, _ := newImageServer(t, http.StatusOK, `{"data":[{"url":"https://cdn.example.com/a.png"}],"cost":0.012}`)

	client := New([]Provider{testProvider(t, server.URL)})

	result, err := client.GenerateImages(context.Background(), "", ImageRequest{Prompt: "a cat"})
	if err != nil {
		t.Fatalf("GenerateImages() error: %v", err)
	}

	if result.Images[0].URL != "https://cdn.example.com/a.png" || result.Images[0].Data != nil {
		t.Errorf("images[0] = %+v, want the URL and no bytes", result.Images[0])
	}

	if result.CostUSD != 0.012 {
		t.Errorf("costUsd = %v, want 0.012", result.CostUSD)
	}

	if result.Provider != "primary" {
		t.Errorf("provider = %q, want the resolved primary", result.Provider)
	}
}

func TestGenerateImagesSelectsTheProvider(t *testing.T) {
	first, firstCaptured := newImageServer(t, http.StatusOK, `{"data":[{"b64_json":"cG5n"}]}`)
	second, secondCaptured := newImageServer(t, http.StatusOK, `{"data":[{"b64_json":"cG5n"}]}`)

	client := New([]Provider{
		{Name: "first", BaseURL: first.URL, APIKey: "sk-first"},
		{Name: "second", BaseURL: second.URL, APIKey: "sk-second"},
	})

	result, err := client.GenerateImages(context.Background(), "", ImageRequest{Prompt: "a cat"})
	if err != nil {
		t.Fatalf("GenerateImages() error: %v", err)
	}

	if result.Provider != "first" || firstCaptured.authorization != "Bearer sk-first" {
		t.Errorf("provider = %q, authorization = %q, want the first provider by default",
			result.Provider, firstCaptured.authorization)
	}

	if secondCaptured.authorization != "" {
		t.Errorf("the second provider was called, want the default to stay on the first")
	}

	result, err = client.GenerateImages(context.Background(), "SECOND", ImageRequest{Prompt: "a cat"})
	if err != nil {
		t.Fatalf("GenerateImages() error: %v", err)
	}

	if result.Provider != "second" || secondCaptured.authorization != "Bearer sk-second" {
		t.Errorf("provider = %q, authorization = %q, want the named provider",
			result.Provider, secondCaptured.authorization)
	}

	if firstCaptured.authorization != "Bearer sk-first" {
		t.Errorf("the first provider was called again, want only the named one")
	}
}

func TestGenerateImagesErrors(t *testing.T) {
	failing, _ := newImageServer(t, http.StatusUnauthorized, `{"error":{"message":"bad key"}}`)
	empty, _ := newImageServer(t, http.StatusOK, `{"data":[]}`)
	undecodable, _ := newImageServer(t, http.StatusOK, `{"data":[{"b64_json":"not base64!"}]}`)
	neither, _ := newImageServer(t, http.StatusOK, `{"data":[{}]}`)

	tests := []struct {
		name         string
		client       *Client
		providerName string
		request      ImageRequest
		want         string
	}{
		{
			name:    "no prompt",
			client:  New([]Provider{testProvider(t, failing.URL)}),
			request: ImageRequest{},
			want:    "prompt is required",
		},
		{
			name:         "unknown provider",
			client:       New([]Provider{testProvider(t, failing.URL)}),
			providerName: "nope",
			request:      ImageRequest{Prompt: "a cat"},
			want:         "no provider named",
		},
		{
			name:    "no providers",
			client:  New(nil),
			request: ImageRequest{Prompt: "a cat"},
			want:    "no providers are configured",
		},
		{
			name:    "http error",
			client:  New([]Provider{testProvider(t, failing.URL)}),
			request: ImageRequest{Prompt: "a cat"},
			want:    "primary: POST /images/generations returned HTTP 401",
		},
		{
			name:    "no images",
			client:  New([]Provider{testProvider(t, empty.URL)}),
			request: ImageRequest{Prompt: "a cat"},
			want:    "primary: answered with no images",
		},
		{
			name:    "undecodable bytes",
			client:  New([]Provider{testProvider(t, undecodable.URL)}),
			request: ImageRequest{Prompt: "a cat"},
			want:    "primary: image 1 of 1: b64_json is not base64",
		},
		{
			name:    "neither shape",
			client:  New([]Provider{testProvider(t, neither.URL)}),
			request: ImageRequest{Prompt: "a cat"},
			want:    "primary: image 1 of 1: carries neither b64_json nor url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.client.GenerateImages(context.Background(), tt.providerName, tt.request)
			if err == nil {
				t.Fatal("GenerateImages() error = nil, want an error")
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.want)
			}
		})
	}
}

func jsonString(value string) string {
	return fmt.Sprintf("%q", value)
}
