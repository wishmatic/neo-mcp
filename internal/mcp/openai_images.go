package mcp

import (
	"context"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/openai"
)

func (c *Clients) openaiRequest(ctx context.Context, in openaiInput) (openai.ImageRequest, error) {
	image, err := c.imageDataURL(ctx, in.Image)
	if err != nil {
		return openai.ImageRequest{}, err
	}

	mask, err := c.imageDataURL(ctx, in.Mask)
	if err != nil {
		return openai.ImageRequest{}, err
	}

	images, err := c.imageDataURLs(ctx, in.Images)
	if err != nil {
		return openai.ImageRequest{}, err
	}

	return openai.ImageRequest{
		Model:  in.Model,
		Prompt: in.Prompt,
		N:      in.N,
		Size:   in.Size,

		Seed:              in.Seed,
		Strength:          in.Strength,
		GuidanceScale:     in.GuidanceScale,
		NumInferenceSteps: in.NumInferenceSteps,

		ImageDataURL:  image,
		ImageDataURLs: images,
		MaskDataURL:   mask,
	}, nil
}

func (c *Clients) imageDataURLs(ctx context.Context, addresses []string) ([]string, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	dataURLs := make([]string, 0, len(addresses))

	for _, address := range addresses {
		dataURL, err := c.imageDataURL(ctx, address)
		if err != nil {
			return nil, err
		}

		dataURLs = append(dataURLs, dataURL)
	}

	return dataURLs, nil
}

// imageDataURL reads an address into the inline form these endpoints take. An address the caller left out resolves to
// none, and the bytes are read here so that the endpoint never fetches anything itself.
func (c *Clients) imageDataURL(ctx context.Context, address string) (string, error) {
	if address == "" {
		return "", nil
	}

	image, err := c.Resolver.Resolve(ctx, address)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}

	return image.DataURL(), nil
}

// generatedImages turns an answer into bytes: the bytes the endpoint sent inline, or the bytes behind the URL it
// answered with instead.
func (c *Clients) generatedImages(ctx context.Context, images []openai.GeneratedImage) ([][]byte, error) {
	out := make([][]byte, 0, len(images))

	for _, image := range images {
		if image.URL == "" {
			out = append(out, image.Data)

			continue
		}

		data, err := c.Resolver.Fetch(ctx, image.URL)
		if err != nil {
			return nil, fmt.Errorf("openai: fetch the linked image instead of the bytes: %w", err)
		}

		out = append(out, data)
	}

	return out, nil
}
