package openai

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	imagesPath = "/images/generations"

	// ResponseFormatBase64 asks for the bytes inline rather than a URL, which is also the API's own default. It is what
	// the client sends when a caller names nothing.
	ResponseFormatBase64 = "b64_json"
)

// ImageRequest is the part of the OpenAI-compatible image request this server exposes. The pointers keep a field a
// caller left out apart from one it set to zero, which some models take as a value.
type ImageRequest struct {
	Model  string `json:"model,omitempty"`
	Prompt string `json:"prompt"`
	N      *int   `json:"n,omitempty"`
	Size   string `json:"size,omitempty"`

	ResponseFormat string `json:"response_format,omitempty"`

	// Inputs travel as data URLs rather than addresses, so the endpoint needs to reach nothing.
	ImageDataURL  string   `json:"imageDataUrl,omitempty"`
	ImageDataURLs []string `json:"imageDataUrls,omitempty"`
	MaskDataURL   string   `json:"maskDataUrl,omitempty"`

	Strength          *float64 `json:"strength,omitempty"`
	GuidanceScale     *float64 `json:"guidance_scale,omitempty"`
	NumInferenceSteps *int     `json:"num_inference_steps,omitempty"`
	Seed              *int     `json:"seed,omitempty"`
}

type imageResponse struct {
	Data []imageData `json:"data"`
	Cost float64     `json:"cost"`
}

type imageData struct {
	URL     string `json:"url"`
	B64JSON string `json:"b64_json"`
}

// GeneratedImage is one image the endpoint produced: the bytes when it answered inline, or the URL it answered with
// instead. It carries one or the other, never both.
type GeneratedImage struct {
	Data []byte
	URL  string
}

type ImageResult struct {
	Provider string
	Images   []GeneratedImage
	CostUSD  float64
}

// GenerateImages posts one prompt to the named provider and returns what came back, or to the first configured
// provider when the caller names none. The endpoint is synchronous, so a caller gets finished images or an error.
func (c *Client) GenerateImages(ctx context.Context, name string, req ImageRequest) (*ImageResult, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	provider, err := c.provider(name)
	if err != nil {
		return nil, err
	}

	result, err := c.images(ctx, provider, req)
	if err != nil {
		return nil, fmt.Errorf("openai: %s: %w", provider.Name, err)
	}

	result.Provider = provider.Name

	return result, nil
}

func (c *Client) images(ctx context.Context, provider Provider, req ImageRequest) (*ImageResult, error) {
	if req.ResponseFormat == "" {
		req.ResponseFormat = ResponseFormatBase64
	}

	var out imageResponse
	if err := c.postJSON(ctx, provider, imagesPath, req, &out); err != nil {
		return nil, err
	}

	return out.result()
}

func (r imageResponse) result() (*ImageResult, error) {
	if len(r.Data) == 0 {
		return nil, fmt.Errorf("answered with no images")
	}

	images := make([]GeneratedImage, 0, len(r.Data))

	for index, entry := range r.Data {
		image, err := entry.image()
		if err != nil {
			return nil, fmt.Errorf("image %d of %d: %w", index+1, len(r.Data), err)
		}

		images = append(images, image)
	}

	return &ImageResult{Images: images, CostUSD: r.Cost}, nil
}

func (d imageData) image() (GeneratedImage, error) {
	switch {
	case d.B64JSON != "":
		data, err := decodeImageData(d.B64JSON)
		if err != nil {
			return GeneratedImage{}, err
		}

		return GeneratedImage{Data: data}, nil
	case d.URL != "":
		return GeneratedImage{URL: d.URL}, nil
	default:
		return GeneratedImage{}, fmt.Errorf("carries neither b64_json nor url")
	}
}

// decodeImageData accepts the data URI prefix some routes wrap these bytes in, and base64 without its padding, both
// of which turn up for this field.
func decodeImageData(encoded string) ([]byte, error) {
	if after, found := strings.CutPrefix(encoded, "data:"); found {
		_, payload, ok := strings.Cut(after, ",")
		if !ok {
			return nil, fmt.Errorf("b64_json starts a data URI it does not finish")
		}

		encoded = payload
	}

	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		data, err := encoding.DecodeString(strings.TrimSpace(encoded))
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("b64_json is not base64")
}

func (r ImageRequest) validate() error {
	if strings.TrimSpace(r.Prompt) == "" {
		return fmt.Errorf("openai: prompt is required")
	}

	return nil
}
