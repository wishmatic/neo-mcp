package sdwebui

import (
	"context"
	"encoding/base64"
	"fmt"
)

var BgkillModels = []string{
	"General",
	"General-HR",
	"General-Lite",
	"General-Lite-2K",
	"Portrait",
	"Matting",
	"Matting-HR",
	"Matting-Lite",
	"Anime-Lite",
	"Dynamic",
	"DIS",
	"HRSOD",
	"COD",
	"DIS-TR_TEs",
}

type BgkillRequest struct {
	ModelName string
	ImageData []byte
	FullMode  bool
}

type birefnetResponse struct {
	OutputImage string `json:"output_image"`
}

func (c *Client) Bgkill(ctx context.Context, req BgkillRequest) ([]byte, error) {
	payload := map[string]any{
		"model_name":        req.ModelName,
		"image":             base64DataURI(req.ImageData),
		"resolution":        "",
		"return_foreground": true,
		"return_mask":       false,
		"return_edge_mask":  false,
		"send_output":       true,
		"use_fp16":          !req.FullMode,
	}

	out, err := postJSON[birefnetResponse](ctx, c, "bgkill", "/birefnet/single", payload)
	if err != nil {
		return nil, err
	}

	if out.OutputImage == "" {
		return nil, fmt.Errorf("bgkill: response contained no foreground image")
	}

	data, err := base64.StdEncoding.DecodeString(out.OutputImage)
	if err != nil {
		return nil, fmt.Errorf("bgkill: decode foreground image: %w", err)
	}

	return data, nil
}
