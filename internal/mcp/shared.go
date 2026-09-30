package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/wishmatic/neo-mcp/internal/format"
)

type formatInput struct {
	Format string `json:"format,omitempty" jsonschema:"output image format: png, jpeg, jxl (JPEG XL), or webp; defaults to the server's configured output format"`
}

// generationOutput is the structured output shared by the image tools. URLs lists the stored images for structured
// clients; the call result's content carries each URL as text alongside the image itself.
type generationOutput struct {
	Count int      `json:"count" jsonschema:"number of images generated"`
	URLs  []string `json:"urls" jsonschema:"URLs for the generated images"`
}

func setDefault(props map[string]*jsonschema.Schema, name string, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal default for %s: %v", name, err))
	}

	props[name].Default = raw
}

func setFormatSchema(s *jsonschema.Schema, def format.Format) {
	setDefault(s.Properties, "format", def.String())
	setFormatEnum(s)
}

func setFormatEnum(s *jsonschema.Schema) {
	names := make([]any, 0, len(format.Names()))
	for _, name := range format.Names() {
		names = append(names, name)
	}

	s.Properties["format"].Enum = names
}
