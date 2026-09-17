package present

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wishmatic/neo-mcp/internal/store"
)

func Examples(model string, examples []store.Example) string {
	if len(examples) == 0 {
		return fmt.Sprintf("No examples saved for %s yet.", model)
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# Examples for %s\n", model)

	for i, example := range examples {
		fmt.Fprintf(&b, "\n## Example %d (%s)\n\n%s\n\n", i+1, example.Tool, jsonBlock(example.Query))
		fmt.Fprintf(
			&b,
			"- Saved: %s\n- Image: %s\n",
			example.CreatedAt.UTC().Format(time.RFC3339),
			example.URL,
		)
	}

	return b.String()
}

func jsonBlock(query string) string {
	indented := indentJSON(query)

	return "```json\n" + indented + "\n```"
}

func indentJSON(query string) string {
	var b bytes.Buffer

	if err := json.Indent(&b, []byte(query), "", "  "); err != nil {
		return query
	}

	return b.String()
}
