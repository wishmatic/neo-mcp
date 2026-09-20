package present

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
)

const (
	RoleUser      mcp.Role = "user"
	RoleAssistant mcp.Role = "assistant"
)

type AttachmentFailure struct {
	Index int
	Err   error
}

// StoredImages presents stored images as [text(url), image, ...], one text block per image immediately before it.
//
// An image that cannot be prepared keeps its URL line and yields a note in place of its image block.
func StoredImages(images [][]byte, urls []string) ([]mcp.Content, []AttachmentFailure) {
	content := make([]mcp.Content, 0, 2*len(urls))
	var failures []AttachmentFailure

	for i, url := range urls {
		content = append(content, &mcp.TextContent{Text: url})

		if i >= len(images) {
			continue
		}

		image, err := imgfmt.Inline(images[i], imgfmt.InlineMaxEdge, imgfmt.InlineMaxBytes)
		if err != nil {
			failures = append(failures, AttachmentFailure{Index: i + 1, Err: err})
			content = append(content, &mcp.TextContent{Text: attachmentFailureNote(i+1, err)})

			continue
		}

		content = append(content, imageBlock(image))
	}

	return content, failures
}

// InlineImage presents an image fetched from url.
func InlineImage(url string, image imgfmt.InlineImage) []mcp.Content {
	return []mcp.Content{
		&mcp.TextContent{Text: fmt.Sprintf("Inline image from %s (%s, %d bytes).", url, image.MediaType, len(image.Data))},
		imageBlock(image),
	}
}

// InlineImageFailure presents a fetched image that could not be prepared for the wire. The call still succeeds.
func InlineImageFailure(url string, err error) []mcp.Content {
	return []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Could not inline %s: %v", url, err)}}
}

// imageBlock annotates the image for both the user and the assistant, which is what lets a vision-capable model see it.
func imageBlock(image imgfmt.InlineImage) *mcp.ImageContent {
	return &mcp.ImageContent{
		Data:        image.Data,
		MIMEType:    image.MediaType,
		Annotations: &mcp.Annotations{Audience: []mcp.Role{RoleUser, RoleAssistant}},
	}
}

func attachmentFailureNote(index int, err error) string {
	return fmt.Sprintf("Image %d could not be attached inline: %v", index, err)
}
