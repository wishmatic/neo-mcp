package present

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
)

// A bounded copy of an image rides in the tool result, so it gets a size budget. WebP is what it is shrunk to because
// that keeps transparency and is a media type the protocol allows.
const (
	DefaultInlineMaxEdge  = 1024
	DefaultInlineMaxBytes = 1 << 20
)

const (
	RoleUser      mcp.Role = "user"
	RoleAssistant mcp.Role = "assistant"
)

// InlineBudget is the size the attached copy of a stored image is shrunk to. The stored image itself is left alone.
//
// A field left at zero or below means the caller named no limit for it, which takes the default.
type InlineBudget struct {
	MaxEdge  int
	MaxBytes int
}

type AttachmentFailure struct {
	Index int
	Err   error
}

// StoredImages presents stored images as [text(url), image, ...], one text block per image immediately before it.
//
// An image that cannot be prepared keeps its URL line and yields a note in place of its image block.
func StoredImages(images [][]byte, urls []string, budget InlineBudget) ([]mcp.Content, []AttachmentFailure) {
	content := make([]mcp.Content, 0, 2*len(urls))
	var failures []AttachmentFailure

	budget = budget.resolved()

	for i, url := range urls {
		content = append(content, &mcp.TextContent{Text: url})

		if i >= len(images) {
			continue
		}

		inline, err := format.Shrink(images[i], budget.MaxEdge, budget.MaxBytes)
		if err != nil {
			failures = append(failures, AttachmentFailure{Index: i + 1, Err: err})
			content = append(content, &mcp.TextContent{Text: attachmentFailureNote(i+1, err)})

			continue
		}

		content = append(content, imageBlock(inline))
	}

	return content, failures
}

func (b InlineBudget) resolved() InlineBudget {
	if b.MaxEdge <= 0 {
		b.MaxEdge = DefaultInlineMaxEdge
	}

	if b.MaxBytes <= 0 {
		b.MaxBytes = DefaultInlineMaxBytes
	}

	return b
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
