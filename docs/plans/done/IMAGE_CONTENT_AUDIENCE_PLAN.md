# Image audience annotations

Status: Done
Depends on: none
Related: `INLINE_IMAGE_RETURN_PLAN.md`, whose `return_as` flag this replaces.

## Goal

Make images reach the assistant by default, controlled by audience annotations rather than by whether the image is
returned at all:

- Every image tool call now stores the image, returns its URL as a text content block, and attaches the image as an MCP
  `image` content block.
- Image blocks carry an `audience` annotation. An omitted audience, or one of only `user`, is shown in the client but
  never reaches the model, so image blocks are always annotated.
- The former `return_as` flag is replaced by a required `visible_to_assistant` boolean. It adds the assistant to the
  audience. The image is always in the user's audience.
- The `inline` tool always annotates its image with `["assistant", "user"]`, because reading the image is its purpose.
- The cost and "vision models only" messaging around the old flag is removed.

## Non-goals

- Removing the stored file or its URL: every call still stores the image and returns the URL.
- Changing `imgfmt.Inline`: downscaling, WebP re-encoding, and the size cap are unchanged.

## Design

### Audience annotations

`mcp.Annotations{Audience: []mcp.Role{...}}` is attached to every image block this service returns:

- generation tools with `visible_to_assistant: false` return `["user"]`;
- generation tools with `visible_to_assistant: true` return `["assistant", "user"]`;
- `inline` returns `["assistant", "user"]`.

The two role strings live in one place (`roleUser`, `roleAssistant`) and the mapping lives in `imageAudience`.

### Content shape

Every generation tool result is `[text(url), image, text(url), image, ...]`, one text block per image carrying its URL
immediately before that image. If an image cannot be encoded, its text failure note takes the place of the image block
and the call still succeeds. The structured `generationOutput` is unchanged.

### The flag

`visible_to_assistant` is a required boolean with no schema default, so a caller cannot silently omit it. It replaces
`return_as` entirely; the previous enum, parser, and schema helper are deleted.

### Text content

Text blocks are `*mcp.TextContent`, which marshals to exactly `{"type":"text","text":"..."}`. A test pins the wire
shape of both the text and image blocks, including the audience annotation.

## Implementation units

### Unit 1: audience annotations on image blocks

Deliverables:

- `internal/mcp/publish.go`: `roleUser` and `roleAssistant`; `imageAudience(visibleToAssistant)`; `imageContent`
  emits paired URL text and annotated image blocks and keeps the fail-soft note.
- `internal/mcp/inline.go`: its image block is annotated for the assistant and user.
- `internal/mcp/publish_test.go`, `inline_test.go`: audience coverage.

Acceptance criteria:

- [x] Generation image blocks are `["user"]` or `["assistant", "user"]` with the flag.
- [x] The `inline` tool's image block is always `["assistant", "user"]`.
- [x] The marshalled content is `type: text` / `type: image` blocks with `mimeType: image/webp` and an `audience`
      annotation, and no base64 in any text block.
- [x] An unencodable image leaves the URL text and a failure note, not an image block, and the call succeeds.
- [x] `go test ./internal/mcp/... -race -count=1` passes.

### Unit 2: replace `return_as` with `visible_to_assistant`

Deliverables:

- `internal/mcp/shared.go`: `audienceInput` with the required boolean; removed `returnInput`, `returnMode`,
  `parseReturnMode`, and `setReturnSchema`.
- `internal/mcp/txt2img.go`, `img2img.go`, `bgkill.go`: embed the flag, drop the mode parse and its schema call, log
  the boolean, and pass it to `publishImages`.
- `internal/mcp/publish.go`: `publishImages` takes the boolean.
- Updated test call sites in `shared_test.go`, `format_test.go`, `provider_test.go`, `examples_test.go`.

Acceptance criteria:

- [x] `visible_to_assistant` is required and boolean with no default in the `txt2img`, `img2img`, and `bgkill` schemas.
- [x] A call that omits it is rejected by schema validation.
- [x] `return_as` appears nowhere in the code or docs outside the superseded plan.
- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

### Unit 3: documentation

Deliverables:

- `README.md`: image calls always store, return the URL, and attach the image; `visible_to_assistant` controls the
  assistant's audience.
- `docs/PROMPT.md`: pass `visible_to_assistant`, set it true to see the result.

Acceptance criteria:

- [x] No token-cost or "vision models only" wording remains for the flag.
- [x] The README describes the always-attached image and the audience flag.

## Verification

- `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.
- Human, LibreChat: a generation with `visible_to_assistant: true` is visible to a vision model, and one with `false`
  is shown to the user only.

## Risks and follow-ups

- A client that ignores annotations will still show every image; the audience is advisory, which is inherent to the
  protocol.
- `visible_to_assistant` being required is a breaking schema change for existing callers.
