package forge

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/utils"
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
	ModelName  string
	ImageData  []byte
	IsFullMode bool
}

type birefnetRequest struct {
	ModelName        string `json:"model_name"`
	Image            string `json:"image"`
	Resolution       string `json:"resolution"`
	ReturnForeground bool   `json:"return_foreground"`
	ReturnMask       bool   `json:"return_mask"`
	ReturnEdgeMask   bool   `json:"return_edge_mask"`
	SendOutput       bool   `json:"send_output"`
	UseFP16          bool   `json:"use_fp16"`
}

type birefnetResponse struct {
	OutputImage string `json:"output_image"`
}

func (c *Client) Bgkill(ctx context.Context, req BgkillRequest) ([]byte, error) {
	out, err := c.postJSON[birefnetResponse](
		ctx,
		"bgkill",
		"/birefnet/single",
		birefnetRequest{
			ModelName:        req.ModelName,
			Image:            utils.Encode(req.ImageData),
			ReturnForeground: true,
			SendOutput:       true,
			UseFP16:          !req.IsFullMode,
		},
	)
	if err != nil {
		return nil, err
	}

	if out.OutputImage == "" {
		return nil, fmt.Errorf("bgkill: response contained no foreground image")
	}

	data, err := base64.StdEncoding.DecodeString(out.OutputImage)
	if err != nil {
		return nil, fmt.Errorf("bgkill: failed to decode foreground image: %w", err)
	}

	return data, nil
}
