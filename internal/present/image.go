package present

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
)

// An attached image rides in the tool result, so it gets a size budget. WebP is what it is shrunk to because that keeps
// transparency and is a media type the protocol allows.
const (
	inlineMaxEdge  = 1024
	inlineMaxBytes = 1 << 20
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

		inline, err := format.Shrink(images[i], inlineMaxEdge, inlineMaxBytes)
		if err != nil {
			failures = append(failures, AttachmentFailure{Index: i + 1, Err: err})
			content = append(content, &mcp.TextContent{Text: attachmentFailureNote(i+1, err)})

			continue
		}

		content = append(content, imageBlock(inline))
	}

	return content, failures
}

// imageBlock annotates the image for both the user and the assistant, which is what lets a vision-capable model see it.
func imageBlock(data []byte) *mcp.ImageContent {
	return &mcp.ImageContent{
		Data:        data,
		MIMEType:    format.WebP.MediaType(),
		Annotations: &mcp.Annotations{Audience: []mcp.Role{RoleUser, RoleAssistant}},
	}
}

func attachmentFailureNote(index int, err error) string {
	return fmt.Sprintf("Image %d could not be attached inline: %v", index, err)
}
